package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/claude/freereps/internal/accounts"
	"github.com/claude/freereps/internal/config"
	"github.com/claude/freereps/internal/storage"
	"github.com/meltforce/meltkit/pkg/auth"
)

// buildAuth constructs pkg/auth with the accounts resolver. It makes no
// network request: OIDC discovery happens on the first login.
func buildAuth(cfg *config.Config, db *storage.DB, log *slog.Logger) (*auth.Auth, *accounts.Accounts, error) {
	accts := accounts.New(db, cfg.Accounts, log)
	a, err := auth.New(*cfg.Auth, auth.NewPgxStore(db.Pool), accts, auth.WithLogger(log))
	if err != nil {
		return nil, nil, err
	}
	accts.Attach(a)
	return a, accts, nil
}

const authUsage = `usage:
  freereps auth create-user [--config path] [--admin] <username>
      creates a password user; the password is read from stdin
  freereps auth create-token [--config path] --user <login> --name <name> --scope api|mcp [--scope …] [--expires 90d]
      prints a personal access token on stdout`

var errUsage = errors.New("usage")

// runAuthCommand runs `freereps auth …` against the configured database. It
// needs an auth: block and a migrated database. Logs go to stderr; stdout
// carries only a created token.
func runAuthCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, authUsage)
		return 2
	}
	var err error
	switch args[0] {
	case "create-user":
		err = cmdCreateUser(args[1:], os.Stdin)
	case "create-token":
		err = cmdCreateToken(args[1:], os.Stdout)
	default:
		err = errUsage
	}
	if errors.Is(err, errUsage) {
		fmt.Fprintln(os.Stderr, authUsage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "freereps auth:", err)
		return 1
	}
	return 0
}

type authEnv struct {
	cfg  *config.Config
	db   *storage.DB
	auth *auth.Auth
}

func openAuthEnv(ctx context.Context, configPath string) (*authEnv, error) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if cfg.Auth == nil {
		return nil, errors.New("the config has no auth: block")
	}
	db, err := storage.New(ctx, cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	a, _, err := buildAuth(cfg, db, log)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &authEnv{cfg: cfg, db: db, auth: a}, nil
}

func cmdCreateUser(args []string, stdin io.Reader) error {
	fs := flag.NewFlagSet("create-user", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path to config file")
	admin := fs.Bool("admin", false, "grant the admin flag")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if fs.NArg() != 1 {
		return errUsage
	}
	username := strings.TrimSpace(fs.Arg(0))

	ctx := context.Background()
	env, err := openAuthEnv(ctx, *configPath)
	if err != nil {
		return err
	}
	defer env.db.Close()
	if !env.cfg.Auth.HasMethod(auth.MethodPassword) {
		return fmt.Errorf("auth.methods does not include %q", auth.MethodPassword)
	}

	if f, ok := stdin.(*os.File); ok && isTerminal(f) {
		fmt.Fprint(os.Stderr, "password: ")
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimRight(line, "\r\n")

	u, err := env.db.CreateUser(ctx, username, username, *admin)
	if err != nil {
		return err
	}
	if err := env.auth.CreatePasswordUser(ctx, int64(u.ID), username, password); err != nil {
		// Leave no user without a login behind.
		_ = env.db.DeleteUser(ctx, u.ID)
		return err
	}
	fmt.Fprintf(os.Stderr, "created user %d (%s), admin=%v\n", u.ID, username, *admin)
	return nil
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func cmdCreateToken(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("create-token", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path to config file")
	login := fs.String("user", "", "login of the token's user")
	name := fs.String("name", "", "token name")
	expires := fs.String("expires", "", "lifetime such as 90d or 720h; empty for none")
	var scopes stringList
	fs.Var(&scopes, "scope", "api or mcp; repeatable")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if *login == "" || *name == "" || len(scopes) == 0 || fs.NArg() != 0 {
		return errUsage
	}
	exp, err := accounts.ParseExpiry(*expires, time.Now())
	if err != nil {
		return err
	}

	ctx := context.Background()
	env, err := openAuthEnv(ctx, *configPath)
	if err != nil {
		return err
	}
	defer env.db.Close()
	u, err := env.db.GetUserByLogin(ctx, *login)
	if err != nil {
		return err
	}
	if u.Disabled() {
		return fmt.Errorf("user %s is disabled", *login)
	}
	secret, _, err := env.auth.CreateToken(ctx, int64(u.ID), *name, scopes, exp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, secret)
	return err
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
