import { expect } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import App from "../App.svelte";
import { FakeApi, settled } from "./fake-api";

export async function renderApp(api = new FakeApi(settled())) {
  const user = userEvent.setup();
  render(App, { props: { api } });
  await waitFor(() => expect(screen.queryByText("Starting…")).not.toBeInTheDocument());
  return { api, user };
}

export function row(name: string): HTMLElement {
  return screen.getByRole("option", { name });
}

// button finds a button by its label or tooltip, inside a row or anywhere.
export function button(label: string, inside?: HTMLElement): HTMLElement {
  return (inside ? within(inside) : screen).getByRole("button", { name: label });
}

export async function settle() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}
