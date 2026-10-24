import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/svelte";
import { FakeApi, sampleDownload } from "./fake-api";
import { button, renderApp, row } from "./render";

const sum = "ab".repeat(32);

function finished(check: Partial<NonNullable<ReturnType<typeof sampleDownload>["check"]>> | null) {
  return sampleDownload({
    id: "iso",
    name: "distro.iso",
    status: "done",
    path: "/home/me/Downloads/distro.iso",
    size: 2048,
    done: 2048,
    finished: Date.now(),
    check: check && {
      algo: "SHA-256",
      weak: false,
      from: "page",
      result: "",
      want: sum,
      got: "",
      error: "",
      read: 0,
      ...check,
    },
  });
}

describe("checksums", () => {
  it("checks a found checksum when asked, and says it matches", async () => {
    const api = new FakeApi([finished({})]);
    const { user } = await renderApp(api);
    const r = row("distro.iso");
    expect(r).toHaveTextContent("SHA-256 found on the page");
    expect(api.calls.some((c) => c.startsWith("check"))).toBe(false);

    await user.click(button("Check", r));
    await waitFor(() => expect(r).toHaveTextContent("SHA-256 matches (from the page)"));
    expect(api.calls).toContain("check iso ");
    expect(screen.queryByRole("button", { name: "Check" })).not.toBeInTheDocument();
  });

  it("offers to download a file that doesn't match again", async () => {
    const api = new FakeApi([finished({})]);
    api.checkResults.iso = "mismatch";
    const { user } = await renderApp(api);
    await user.click(button("Check", row("distro.iso")));
    await waitFor(() =>
      expect(row("distro.iso")).toHaveTextContent(
        "Doesn't match its SHA-256: the file may be damaged",
      ),
    );

    await user.click(button("Download again", row("distro.iso")));
    await waitFor(() => expect(api.calls).toContain("redownload iso"));
    expect(await screen.findByText("Downloading distro.iso again")).toBeInTheDocument();
  });

  it("checks against a pasted checksum, the clipboard's filled in", async () => {
    const api = new FakeApi([finished(null)]);
    api.clipboard = `sha256:${sum}`;
    const { user } = await renderApp(api);
    expect(row("distro.iso")).not.toHaveTextContent("SHA-256");
    expect(screen.queryByRole("button", { name: "Check" })).not.toBeInTheDocument();

    await user.pointer({ keys: "[MouseRight]", target: row("distro.iso") });
    await user.click(screen.getByRole("menuitem", { name: "Check checksum…" }));
    const dialog = screen.getByRole("dialog", { name: "Check checksum" });
    expect(dialog).toHaveTextContent("Snag found no checksum for this file");
    await waitFor(() => expect(screen.getByLabelText("Checksum")).toHaveValue(`sha256:${sum}`));

    await user.click(button("Check", dialog));
    await waitFor(() => expect(api.calls).toContain(`check iso sha256:${sum}`));
    await waitFor(() => expect(dialog).toHaveTextContent("SHA-256 matches (the one you pasted)"));
  });

  it("refuses what isn't a checksum, and copies the file's own", async () => {
    const api = new FakeApi([finished(null)]);
    const { user } = await renderApp(api);
    await user.pointer({ keys: "[MouseRight]", target: row("distro.iso") });
    await user.click(screen.getByRole("menuitem", { name: "Check checksum…" }));
    const dialog = screen.getByRole("dialog", { name: "Check checksum" });
    expect(button("Check", dialog)).toBeDisabled();

    await user.type(screen.getByLabelText("Checksum"), "hello");
    expect(dialog).toHaveTextContent("That isn't a checksum Snag knows.");
    expect(button("Check", dialog)).toBeDisabled();

    await user.click(button("Copy SHA-256", dialog));
    await waitFor(() => expect(api.clipboard).toBe(api.fileChecksum));
    expect(await screen.findByText("SHA-256 copied")).toBeInTheDocument();
  });
});
