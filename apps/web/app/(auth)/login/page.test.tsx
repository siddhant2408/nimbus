import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

function createWrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

const { mockPush, mockReplace, searchParamsState, authStateRef } = vi.hoisted(
  () => ({
    mockPush: vi.fn(),
    mockReplace: vi.fn(),
    searchParamsState: { params: new URLSearchParams() },
    authStateRef: {
      state: {
        user: null as null | { id: string; email: string; onboarded_at: null },
        isLoading: false,
        setUser: vi.fn(),
      },
    },
  }),
);

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  usePathname: () => "/login",
  useSearchParams: () => searchParamsState.params,
}));

vi.mock("@nimbus/core/auth", async () => {
  const actual =
    await vi.importActual<typeof import("@nimbus/core/auth")>(
      "@nimbus/core/auth",
    );
  const useAuthStore = Object.assign(
    (selector: (s: typeof authStateRef.state) => unknown) =>
      selector(authStateRef.state),
    { getState: () => authStateRef.state },
  );
  return { ...actual, useAuthStore };
});

vi.mock("@/features/auth/auth-cookie", () => ({
  setLoggedInCookie: vi.fn(),
}));

vi.mock("@nimbus/core/api", () => ({
  api: {
    listWorkspaces: vi.fn().mockResolvedValue([]),
    listMyInvitations: vi.fn().mockResolvedValue([]),
    setToken: vi.fn(),
  },
}));

vi.mock("@nimbus/core/paths", async () => {
  const actual =
    await vi.importActual<typeof import("@nimbus/core/paths")>(
      "@nimbus/core/paths",
    );
  return {
    ...actual,
    useHasOnboarded: () => false,
  };
});

const makeUser = () => ({
  id: "user-1",
  name: "Test",
  email: "test@example.com",
  avatar_url: null,
  onboarded_at: null,
  onboarding_questionnaire: {},
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
});

import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParamsState.params = new URLSearchParams();
    authStateRef.state.user = null;
    authStateRef.state.isLoading = false;
    global.fetch = vi.fn();
  });

  it("renders email input and continue button", () => {
    render(<LoginPage />, { wrapper: createWrapper() });
    expect(screen.getByText("Sign in to Nimbus")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Continue" })).toBeInTheDocument();
  });

  it("continue button is disabled when email is empty", () => {
    render(<LoginPage />, { wrapper: createWrapper() });
    expect(screen.getByRole("button", { name: "Continue" })).toBeDisabled();
  });

  it("POSTs to /auth/login on submit and redirects", async () => {
    const user = userEvent.setup();
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ token: "tok", user: makeUser() }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    render(<LoginPage />, { wrapper: createWrapper() });
    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.click(screen.getByRole("button", { name: "Continue" }));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/auth/login",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ email: "test@example.com" }),
        }),
      );
    });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalled();
    });
  });

  it("shows 'Signing in...' while the request is pending", async () => {
    const user = userEvent.setup();
    vi.mocked(global.fetch).mockReturnValueOnce(new Promise(() => {}));

    render(<LoginPage />, { wrapper: createWrapper() });
    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.click(screen.getByRole("button", { name: "Continue" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Signing in..." })).toBeInTheDocument();
    });
  });

  it("shows error message when login fails", async () => {
    const user = userEvent.setup();
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Email not allowed" }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      }),
    );

    render(<LoginPage />, { wrapper: createWrapper() });
    await user.type(screen.getByLabelText("Email"), "blocked@example.com");
    await user.click(screen.getByRole("button", { name: "Continue" }));

    await waitFor(() => {
      expect(screen.getByText("Email not allowed")).toBeInTheDocument();
    });
  });
});
