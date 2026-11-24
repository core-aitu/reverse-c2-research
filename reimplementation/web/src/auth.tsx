import {
  createContext,
  type FormEvent,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { AuthResponse, Identity } from "./types";

type AuthContextValue = {
  identity: Identity | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  request: <T>(path: string, init?: RequestInit) => Promise<T>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

async function responseJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? `Request failed (${response.status})`);
  }
	if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState("");
  const [identity, setIdentity] = useState<Identity | null>(null);
  const [loading, setLoading] = useState(true);

  const applyAuth = useCallback((auth: AuthResponse) => {
    setToken(auth.jwtToken);
    setIdentity({ username: auth.username, roles: auth.roles });
  }, []);

  const refresh = useCallback(async () => {
    const response = await fetch("/api/Accounts/refresh-token", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    });
    const auth = await responseJSON<AuthResponse>(response);
    applyAuth(auth);
    return auth.jwtToken;
  }, [applyAuth]);

  useEffect(() => {
    refresh().catch(() => undefined).finally(() => setLoading(false));
  }, [refresh]);

  const login = useCallback(
    async (username: string, password: string) => {
      const response = await fetch("/api/Accounts/authenticate", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      applyAuth(await responseJSON<AuthResponse>(response));
    },
    [applyAuth],
  );

  const logout = useCallback(async () => {
    if (token) {
      await fetch("/api/Accounts/revoke-token", {
        method: "POST",
        credentials: "include",
        headers: { Authorization: `Bearer ${token}` },
      }).catch(() => undefined);
    }
    setToken("");
    setIdentity(null);
  }, [token]);

  const request = useCallback(
    async <T,>(path: string, init: RequestInit = {}) => {
      const perform = (accessToken: string) =>
        fetch(path, {
          ...init,
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            ...init.headers,
            Authorization: `Bearer ${accessToken}`,
          },
        });
      let response = await perform(token);
      if (response.status === 401) {
        const renewed = await refresh();
        response = await perform(renewed);
      }
      return responseJSON<T>(response);
    },
    [refresh, token],
  );

  const value = useMemo(
    () => ({ identity, loading, login, logout, request }),
    [identity, loading, login, logout, request],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider is missing");
  return value;
}

export function Login() {
  const { login } = useAuth();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      await login(String(data.get("username")), String(data.get("password")));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="login-shell">
      <section className="login-card">
        <div className="brand-mark">SI</div>
        <p className="eyebrow">Clean-room research environment</p>
        <h1>Specter Research Console</h1>
        <p className="muted">Authenticate to inspect the reconstructed management plane.</p>
        <form onSubmit={submit}>
          <label>Username<input name="username" autoComplete="username" required /></label>
          <label>Password<input name="password" type="password" autoComplete="current-password" required /></label>
          {error && <p className="error" role="alert">{error}</p>}
          <button className="primary" disabled={busy}>{busy ? "Authenticating…" : "Sign in"}</button>
        </form>
      </section>
    </main>
  );
}
