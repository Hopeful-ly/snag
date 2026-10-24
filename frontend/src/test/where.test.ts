import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/svelte";
import { FakeApi, sampleDownload, sampleQueues } from "./fake-api";
import { button, renderApp } from "./render";

describe("somewhere else", () => {
  function missed() {
    const api = new FakeApi([
      sampleDownload({
        id: "bitab",
        name: "Mehrad Hidden - Bitab",
        status: "error",
        where: true,
        error: "couldn't find “Bitab” by Mehrad Hidden on YouTube Music: no match",
        queue: "night",
      }),
    ]);
    api.queuesValue = sampleQueues();
    api.searchValue = [
      {
        title: "Bitab (Live)",
        by: "Mehrad Hidden",
        seconds: 270,
        url: "https://www.youtube.com/watch?v=live",
        match: false,
      },
      {
        title: "Bitab",
        by: "Mehrad Hidden",
        seconds: 201,
        url: "https://www.youtube.com/watch?v=studio",
        match: true,
      },
    ];
    return api;
  }

  it("uses the result they pick, and can keep that place for the other songs", async () => {
    const api = missed();
    const { user } = await renderApp(api);
    expect(screen.getByText("Try again on YouTube Music, or somewhere else")).toBeInTheDocument();

    await user.click(button("Try again"));
    const dialog = screen.getByRole("dialog", { name: "Try again" });
    expect(dialog).toHaveTextContent("Mehrad Hidden - Bitab");

    await user.click(button("Another source", dialog));
    await user.click(button("YouTube", dialog));
    await waitFor(() => expect(api.calls).toContain("searchElsewhere bitab youtube"));
    expect(screen.getByRole("radio", { name: /Matches/ })).toHaveAttribute("aria-checked", "true");

    await user.click(screen.getByRole("checkbox", { name: /other songs/ }));
    await user.click(button("Use this"));
    await waitFor(() =>
      expect(api.calls).toContain(
        "useFound bitab https://www.youtube.com/watch?v=studio youtube true",
      ),
    );
    expect(api.queuesValue.find((q) => q.id === "night")?.fallback).toBe("youtube");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("opens the question when a song has just failed to match", async () => {
    const api = new FakeApi([sampleDownload({ id: "ok", name: "done.mp3", status: "done" })]);
    await renderApp(api);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    api.emit(
      sampleDownload({
        id: "bitab",
        name: "Mehrad Hidden - Bitab",
        status: "error",
        where: true,
        error: "no match",
      }),
    );
    expect(await screen.findByRole("dialog", { name: "Try again" })).toBeInTheDocument();
  });

  it("retries YouTube Music when they say so", async () => {
    const api = missed();
    const { user } = await renderApp(api);
    await user.click(button("Try again"));
    await user.click(button("Retry YouTube Music"));
    await waitFor(() => expect(api.calls).toContain("resume bitab"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("takes a link they paste", async () => {
    const api = missed();
    const { user } = await renderApp(api);
    await user.click(button("Try again"));
    const dialog = screen.getByRole("dialog", { name: "Try again" });
    await user.click(button("Another source", dialog));
    await user.type(screen.getByLabelText("Or paste a link"), "https://soundcloud.com/ashe/stop");
    await user.click(button("Use this", dialog));
    await waitFor(() =>
      expect(api.calls).toContain("useFound bitab https://soundcloud.com/ashe/stop  false"),
    );
  });
});
