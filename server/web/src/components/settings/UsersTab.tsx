import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchMe, fetchUsers, updateUser } from "../../api";
import { ITEM, formatWhen } from "./AccountTab";
import { MONO, TabHeader } from "./parts";

export default function UsersTab() {
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });
  const users = useQuery({ queryKey: ["users"], queryFn: fetchUsers });
  const update = useMutation({
    mutationFn: (v: {
      id: number;
      patch: { is_admin?: boolean; disabled?: boolean };
    }) => updateUser(v.id, v.patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });

  return (
    <>
      <TabHeader title="Users">
        Every account on this server. An admin manages these accounts; a
        disabled account can no longer sign in, and its data stays stored.
      </TabHeader>

      {users.error ? (
        <p
          style={{
            color: "var(--color-accent-700)",
            fontSize: 13,
            paddingTop: 16,
          }}
        >
          {users.error.message}
        </p>
      ) : null}

      {users.data?.map((u) => {
        // /me carries no user id; the login is unique per account.
        const self = u.login === me.data?.login;
        return (
          <div key={u.id} style={ITEM}>
            <div style={{ minWidth: 0, flex: "1 1 240px" }}>
              <div
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  alignItems: "center",
                  gap: 8,
                  fontSize: 14,
                }}
              >
                <span style={{ overflowWrap: "anywhere" }}>
                  {u.display_name || u.login}
                </span>
                {u.is_admin ? (
                  <span className="tag tag-accent">Admin</span>
                ) : null}
                {u.disabled_at ? (
                  <span className="tag tag-outline">Disabled</span>
                ) : null}
                {self ? <span className="tag tag-neutral">You</span> : null}
              </div>
              <div
                style={{
                  font: "400 12px/1.5 var(--font-body)",
                  color: "var(--color-neutral-600)",
                  marginTop: 3,
                }}
              >
                <span style={MONO}>{u.login}</span> · last seen{" "}
                {formatWhen(u.last_seen)}
              </div>
            </div>
            {/* The server refuses changes to the own account with 409. */}
            {!self ? (
              <span style={{ display: "flex", gap: 8 }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ fontSize: 12 }}
                  disabled={update.isPending}
                  onClick={() =>
                    update.mutate({
                      id: u.id,
                      patch: { is_admin: !u.is_admin },
                    })
                  }
                >
                  {u.is_admin ? "Remove admin" : "Make admin"}
                </button>
                <button
                  type="button"
                  className="btn btn-ghost"
                  style={{ fontSize: 12 }}
                  disabled={update.isPending}
                  onClick={() =>
                    update.mutate({
                      id: u.id,
                      patch: { disabled: !u.disabled_at },
                    })
                  }
                >
                  {u.disabled_at ? "Enable" : "Disable"}
                </button>
              </span>
            ) : null}
          </div>
        );
      })}

      {update.error ? (
        <p
          style={{
            color: "var(--color-accent-700)",
            fontSize: 13,
            paddingTop: 12,
          }}
        >
          {update.error.message}
        </p>
      ) : null}
    </>
  );
}
