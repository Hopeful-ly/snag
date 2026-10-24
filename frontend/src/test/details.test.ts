import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, sampleDownload } from "./fake-api";
import { button, renderApp, row } from "./render";

const GB = 2 ** 30;

function episode(fields: Parameters<typeof sampleDownload>[0]) {
  return sampleDownload({
    id: "e1",
    name: "Cyberpunk Edgerunners S02E01.mp4",
    kind: "stream",
    status: "downloading",
    size: 1.1 * GB,
    done: 1.09 * GB,
    estimated: true,
    speed: 5 * 1024,
    eta: 975,
    pieces: 415,
    piecesDone: 412,
    heard: Date.now(),
    ...fields,
  });
}

const panel = (name: string) => screen.getByRole("complementary", { name: `Details of ${name}` });

describe("a download's details", () => {
  it("opens on a click, and says a stream at 99% is being finished", async () => {
    const api = new FakeApi([
      episode({
        stage: "Fixing the video's container",
        stageDone: 450,
        stageOf: 1000,
        speed: 0,
        eta: null,
      }),
    ]);
    api.activityValue.e1 = [
      { at: Date.now() - 4000, text: "[download] 100% of ~1.10GiB in 00:12:31" },
      { at: Date.now() - 3000, text: "Fixing the video's container…" },
      {
        at: Date.now() - 3000,
        text: '[FixupM3u8] Fixing MPEG-TS in MP4 container of "S02E01.mp4"',
      },
    ];
    const { user } = await renderApp(api);
    const r = row("Cyberpunk Edgerunners S02E01.mp4");
    expect(r).toHaveTextContent("Fixing the video's container… 45%");
    expect(r).not.toHaveTextContent("left");

    await user.click(r);
    const p = panel("Cyberpunk Edgerunners S02E01.mp4");
    expect(within(p).getByRole("status")).toHaveTextContent("Finishing 45%");
    expect(p).toHaveTextContent("this step works through the whole file");
    expect(p).toHaveTextContent("412 of 415");
    expect(p).not.toHaveTextContent("Speed");
    await waitFor(() => expect(p).toHaveTextContent("[FixupM3u8] Fixing MPEG-TS in MP4 container"));

    await user.click(button("Close (Esc)", p));
    expect(screen.queryByRole("complementary")).toBeNull();
  });

  it("says when the site has gone quiet", async () => {
    const api = new FakeApi([episode({ heard: Date.now() - 4 * 60_000 })]);
    const { user } = await renderApp(api);
    await user.click(row("Cyberpunk Edgerunners S02E01.mp4"));
    const p = panel("Cyberpunk Edgerunners S02E01.mp4");
    expect(within(p).getByRole("status")).toHaveTextContent("Waiting for the site");
    expect(p).toHaveTextContent("Nothing has come for 4 min");
    await user.click(button("Pause", p));
    await waitFor(() => expect(api.calls).toContain("pause e1"));
  });

  it("follows the selection, and opens from the menu", async () => {
    const api = new FakeApi([
      episode({}),
      sampleDownload({
        id: "f",
        name: "manual.pdf",
        status: "done",
        path: "/home/me/Downloads/manual.pdf",
        size: 1000,
        done: 1000,
        finished: Date.now(),
      }),
    ]);
    const { user } = await renderApp(api);
    await user.pointer({ keys: "[MouseRight]", target: row("manual.pdf") });
    await user.click(screen.getByRole("menuitem", { name: "Details" }));
    expect(within(panel("manual.pdf")).getByRole("status")).toHaveTextContent("Done");
    expect(panel("manual.pdf")).toHaveTextContent("~/Downloads/manual.pdf");
    await user.keyboard("{ArrowUp}");
    expect(panel("Cyberpunk Edgerunners S02E01.mp4")).toBeInTheDocument();
  });
});
