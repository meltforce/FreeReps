import { Component, ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div className="bg-zinc-900 border border-red-900/50 rounded-lg p-6">
          <h3 className="text-red-400 font-medium mb-2">
            Something went wrong
          </h3>
          <p className="text-zinc-500 text-sm">
            {this.state.error?.message ?? "An unexpected error occurred."}
          </p>
          <button
            onClick={() => this.setState({ hasError: false, error: null })}
            className="mt-3 px-3 py-1.5 bg-zinc-800 text-zinc-300 rounded-md text-sm hover:bg-zinc-700"
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
