import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import PlaylistWindow from "../PlaylistWindow.svelte";
import { FakeApi, sampleQueues, settled } from "./fake-api";
import { button } from "./render";
import { NameKind, Via } from "../../bindings/github.com/Roham1299/snag/internal/playlist/models";

async function openPlaylistWindow(api = new FakeApi(settled())) {
  const user = userEvent.setup();
  render(PlaylistWindow, { props: { api, id: "p1" } });
  await screen.findByRole("heading", { name: "Lanterns: Season 1" });
  return { api, user, main: screen.getByRole("main", { name: "Download playlist" }) };
}

const entry = (name: string) => screen.getByRole("checkbox", { name });
const nameFields = (main: HTMLElement) =>
  within(within(main).getByRole("group", { name: "Name fields" }))
    .getAllByRole("button")
    .map((b) => b.textContent);
const ticked = () =>
  within(screen.getByRole("group", { name: "Entries" }))
    .getAllByRole("checkbox")
    .filter((box) => box.getAttribute("aria-checked") === "true")
    .map((box) => box.getAttribute("aria-label"));

describe("the Download playlist window", () => {
  it("shows the list with the rest of it ticked from the playing one", async () => {
    const { main } = await openPlaylistWindow();
    expect(
      within(main).getByText("v.example.com · 4 videos · over 1 h 15 min"),
    ).toBeInTheDocument();
    expect(ticked()).toEqual(["Episode 2", "Episode 3", "4"]); // the last is named by its address
    expect(within(entry("Episode 2")).getByText("Playing")).toBeInTheDocument();
    expect(within(entry("Episode 1")).getByText("25:00")).toBeInTheDocument();
    expect(button("Download 3", main)).toBeEnabled();
  });

  it("leaves out what Snag has, and marks it", async () => {
    const api = new FakeApi(settled());
    api.playlistHaveValue = { best: [false, false, true, false] };
    await openPlaylistWindow(api);
    expect(ticked()).toEqual(["Episode 2", "4"]);
    expect(within(entry("Episode 3")).getByText("In Snag")).toBeInTheDocument();
  });

  it("picks all, none, one, and a range with shift", async () => {
    const { user, main } = await openPlaylistWindow();
    await user.click(button("None", main));
    expect(ticked()).toEqual([]);
    expect(button("Download 0", main)).toBeDisabled();
    await user.click(button("All", main));
    expect(ticked()).toHaveLength(4);
    await user.click(entry("Episode 1"));
    expect(ticked()).toEqual(["Episode 2", "Episode 3", "4"]);
    await user.click(entry("Episode 3"));
    await user.keyboard("{Shift>}");
    await user.click(entry("4"));
    await user.keyboard("{/Shift}");
    expect(ticked()).toEqual(["Episode 2"]);
    await user.click(button("From the playing one on", main));
    expect(ticked()).toEqual(["Episode 2", "Episode 3", "4"]);
  });

  it("reverses the list so the last entry downloads first", async () => {
    const { api, user, main } = await openPlaylistWindow();
    const labels = () =>
      within(within(main).getByRole("group", { name: "Entries" }))
        .getAllByRole("checkbox")
        .map((box) => box.getAttribute("aria-label"));
    expect(labels()).toEqual(["Episode 1", "Episode 2", "Episode 3", "4"]);
    await user.click(button("Reverse", main));
    expect(button("Reverse", main)).toHaveAttribute("aria-pressed", "true");
    expect(labels()).toEqual(["4", "Episode 3", "Episode 2", "Episode 1"]);
    expect(within(entry("Episode 2")).getByText("Playing")).toBeInTheDocument();
    await user.click(button("Download 3", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'downloadPlaylist p1 {"entries":[3,2,1],"quality":"best","directory":"/home/me/Downloads/Lanterns Season 1","queue":"","naming":"{show} S{season}E{episode} - {title}","follow":false,"everyHours":6}',
      ),
    );
  });

  it("ticks or unticks a season at once", async () => {
    const { user, main } = await openPlaylistWindow();
    await user.click(button("None", main));
    await user.click(button("Season 2", main));
    expect(ticked()).toEqual(["Episode 3", "4"]);
    expect(button("Season 2", main)).toHaveAttribute("aria-pressed", "true");
    await user.click(button("Season 1", main));
    expect(ticked()).toHaveLength(4);
    await user.click(button("Season 2", main)); // all of it ticked: unticks it
    expect(ticked()).toEqual(["Episode 1", "Episode 2"]);
  });

  it("ticks what a press sweeps over, quick moves too", async () => {
    const { main } = await openPlaylistWindow();
    // Rows 34 high from y 100: Episode 1 at 100, 2 at 134, 3 at 168, 4 at 202.
    const rows = within(main).getAllByRole("checkbox");
    rows[0]!.getBoundingClientRect = () => ({ top: 100, height: 34 }) as DOMRect;
    await fireEvent.pointerDown(rows[0]!, { button: 0, pointerId: 1, clientY: 110 });
    expect(ticked()).toEqual(["Episode 1", "Episode 2", "Episode 3", "4"]); // it was unticked
    await fireEvent.pointerMove(window, { pointerId: 1, clientY: 215 }); // a quick move to the 4th
    await fireEvent.pointerUp(window, { pointerId: 1, clientY: 215 });
    expect(ticked()).toEqual(["Episode 1", "Episode 2", "Episode 3", "4"]);

    await fireEvent.pointerDown(rows[3]!, { button: 0, pointerId: 2, clientY: 210 }); // unticks
    await fireEvent.pointerMove(window, { pointerId: 2, clientY: 140 }); // back up to the 2nd
    await fireEvent.pointerUp(window, { pointerId: 2 });
    await fireEvent.pointerMove(window, { pointerId: 2, clientY: 105 }); // let go: no more
    expect(ticked()).toEqual(["Episode 1"]);
  });

  it("shows a song's file name, with who it features in the title and the list name beside it", async () => {
    const api = new FakeApi(settled());
    const shown =
      "Daft Punk, Pharrell Williams, Nile Rodgers - Get Lucky (feat. Pharrell Williams and Nile Rodgers)";
    api.playlistValue = {
      ...api.playlistValue,
      title: "Random Access Memories",
      kind: NameKind.KindSongs,
      quality: "mp3",
      naming: "{artist} - {title}",
      entries: [
        {
          url: "https://open.spotify.com/track/1",
          title: shown,
          duration: 369,
          tags: {
            title: "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
            artist: "Daft Punk, Pharrell Williams, Nile Rodgers",
            album_artist: "Daft Punk",
          },
        },
      ],
    };
    render(PlaylistWindow, { props: { api, id: "p1" } });
    const row = await screen.findByRole("checkbox", { name: shown });
    expect(
      within(row).getByText("Daft Punk - Get Lucky (feat. Pharrell Williams and Nile Rodgers)"),
    ).toBeInTheDocument();
    expect(within(row).getByText(shown)).toBeInTheDocument();
  });

  it("shows the formatted name, with the list's name beside it", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = {
      ...api.playlistValue,
      entries: api.playlistValue.entries.map((entry, i) =>
        i === 0
          ? {
              ...entry,
              title: "S1 E1 · Pilot",
              tags: { title: "Pilot", show: "Lanterns", season: 1, episode: 1 },
            }
          : entry,
      ),
    };
    const { main } = await openPlaylistWindow(api);
    const row = within(main).getByRole("checkbox", { name: "S1 E1 · Pilot" });
    expect(within(row).getByText("Lanterns S01E01 - Pilot")).toBeInTheDocument();
    expect(within(row).getByText("S1 E1 · Pilot")).toBeInTheDocument();
  });

  it("offers a show's fields, not a song's", async () => {
    const { main } = await openPlaylistWindow();
    expect(nameFields(main)).toEqual(["{show}", "{season}", "{episode}", "{title}"]);
  });

  it("offers video fields, and song fields once the quality is audio", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = { ...api.playlistValue, kind: NameKind.KindVideos };
    const { user, main } = await openPlaylistWindow(api);
    expect(nameFields(main)).toEqual(["{n}", "{title}", "{artist}", "{playlist}"]);
    await user.click(within(main).getByRole("radio", { name: "Audio MP3" }));
    expect(nameFields(main)).toEqual([
      "{artist}",
      "{title}",
      "{album}",
      "{track}",
      "{year}",
      "{n}",
      "{playlist}",
    ]);
  });

  it("downloads the ones picked, at the quality, folder and queue chosen", async () => {
    const api = new FakeApi(settled());
    api.queuesValue = sampleQueues();
    const { user, main } = await openPlaylistWindow(api);
    expect(
      within(main).queryByRole("switch", { name: "In a folder of its own" }),
    ).not.toBeInTheDocument();
    expect(within(main).getByLabelText("Save to")).toHaveValue(
      "/home/me/Downloads/Lanterns Season 1",
    );
    await user.click(within(main).getByRole("radio", { name: "720p" }));
    await user.click(within(main).getByRole("button", { name: "~/Videos" }));
    await user.click(within(main).getByRole("radio", { name: "Tonight" }));
    await user.click(button("Download 3", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'downloadPlaylist p1 {"entries":[1,2,3],"quality":"720","directory":"/home/me/Videos/Lanterns Season 1","queue":"night","naming":"{show} S{season}E{episode} - {title}","follow":false,"everyHours":6}',
      ),
    );
  });

  it("makes a queue named after the list, or uses the one there is", async () => {
    const { api, user, main } = await openPlaylistWindow();
    expect(within(main).getByRole("radio", { name: "New: Lanterns: Season 1" })).toBeChecked();
    await user.click(button("Download 3", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'downloadPlaylist p1 {"entries":[1,2,3],"quality":"best","directory":"/home/me/Downloads/Lanterns Season 1","queue":"","naming":"{show} S{season}E{episode} - {title}","follow":false,"everyHours":6}',
      ),
    );
  });

  it("picks the list's queue when there is one", async () => {
    const api = new FakeApi(settled());
    api.queuesValue = [
      ...sampleQueues(),
      { ...sampleQueues()[2]!, id: "q9", name: "lanterns: season 1" },
    ];
    const { main } = await openPlaylistWindow(api);
    expect(within(main).queryByRole("radio", { name: /^New/ })).not.toBeInTheDocument();
    expect(within(main).getByRole("radio", { name: "lanterns: season 1" })).toBeChecked();
  });

  it("says what went otherwise, and closes when told", async () => {
    const api = new FakeApi(settled());
    api.playlistNote = "They are in Main: there can be 20 queues besides Main; delete one first";
    const { user, main } = await openPlaylistWindow(api);
    await user.click(button("Download 3", main));
    expect(await within(main).findByRole("status")).toHaveTextContent("They are in Main");
    await user.click(button("Done", main));
    expect(api.calls).toContain("closeWindow");
  });

  it("shows a failure in place", async () => {
    const api = new FakeApi(settled());
    api.failing.set("downloadPlaylist", "that playlist is gone; open it again from its page");
    const { user, main } = await openPlaylistWindow(api);
    await user.click(button("Download 3", main));
    expect(await within(main).findByRole("alert")).toHaveTextContent("that playlist is gone");
  });

  it("closes with Escape", async () => {
    const { api, user } = await openPlaylistWindow();
    await user.keyboard("{Escape}");
    expect(api.calls).toContain("closeWindow");
  });
});

describe("a streaming site's season", () => {
  it("says how its episodes are fetched, and offers the qualities too", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = { ...api.playlistValue, via: Via.ViaBrowser };
    const { user, main } = await openPlaylistWindow(api);
    expect(within(main).getByText(/background tab of your browser/)).toBeInTheDocument();
    expect(within(entry("Episode 1")).getByText("~25 min")).toBeInTheDocument(); // not "25:00"
    await user.click(within(main).getByRole("radio", { name: "720p" }));
    await user.click(button("Download 3", main));
    await waitFor(() =>
      expect(
        api.calls.some((c) =>
          c.startsWith('downloadPlaylist p1 {"entries":[1,2,3],"quality":"720"'),
        ),
      ).toBe(true),
    );
  });
});

describe("a Spotify list", () => {
  it("says where its songs come from and what is missing, with their exact lengths", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = {
      ...api.playlistValue,
      via: Via.ViaSearch,
      note: "Spotify shows 100 of its songs to Snag on its own.",
    };
    const { main } = await openPlaylistWindow(api);
    expect(within(main).getByText(/found on YouTube Music/)).toBeInTheDocument();
    expect(within(main).getByText(/shows 100 of its songs/)).toBeInTheDocument();
    expect(within(entry("Episode 1")).getByText("25:00")).toBeInTheDocument();
  });

  it("saves a song alone with the rest of the downloads", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = {
      ...api.playlistValue,
      title: "Ashe - Stop The Wedding!",
      via: Via.ViaSearch,
      current: -1,
      folder: "/home/me/Downloads",
      folderName: "",
      kind: NameKind.KindSongs,
      quality: "mp3",
      naming: "{artist} - {title}",
      entries: [
        {
          url: "https://open.spotify.com/track/1",
          title: "Ashe - Stop The Wedding!",
          duration: 199,
        },
      ],
    };
    const user = userEvent.setup();
    render(PlaylistWindow, { props: { api, id: "p1" } });
    const main = await screen.findByRole("main", { name: "Download playlist" });
    await screen.findByRole("heading", { name: "Ashe - Stop The Wedding!" });
    await user.click(button("Download 1", main));
    await waitFor(() =>
      expect(
        api.calls.some(
          (c) =>
            c.startsWith("downloadPlaylist p1") &&
            c.includes('"directory":"/home/me/Downloads"') &&
            c.includes('"queue":"main"') &&
            c.includes('"naming":"{artist} - {title}"'),
        ),
      ).toBe(true),
    );
  });

  it("follows the list, downloading only what comes after", async () => {
    const api = new FakeApi(settled());
    api.startAtLoginValue = { on: false, available: true, desktop: "", command: "" };
    const { user, main } = await openPlaylistWindow(api);
    await user.click(button("None", main));
    expect(button("Download 0", main)).toBeDisabled();
    await user.click(screen.getByRole("checkbox", { name: "Keep downloading new ones" }));
    await user.click(screen.getByRole("radio", { name: "Every day" }));
    expect(main).toHaveTextContent("Snag checks for new ones only while it's running");
    await user.click(button("Turn it on", main));
    await waitFor(() => expect(api.calls).toContain("setStartAtLogin true"));
    expect(main).not.toHaveTextContent("Snag checks for new ones only while it's running");

    await user.click(button("Subscribe", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'downloadPlaylist p1 {"entries":[],"quality":"best","directory":"/home/me/Downloads/Lanterns Season 1","queue":"","naming":"{show} S{season}E{episode} - {title}","follow":true,"everyHours":24}',
      ),
    );
  });

  it("opens ready to follow from the card's Subscribe, nothing ticked", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = { ...api.playlistValue, follow: true };
    const { main } = await openPlaylistWindow(api);
    expect(ticked()).toEqual([]);
    expect(screen.getByRole("checkbox", { name: "Keep downloading new ones" })).toBeChecked();
    expect(button("Subscribe", main)).toBeEnabled();
  });

  it("can't follow a streaming site's show", async () => {
    const api = new FakeApi(settled());
    api.playlistValue = { ...api.playlistValue, canFollow: false };
    await openPlaylistWindow(api);
    expect(screen.queryByRole("checkbox", { name: "Keep downloading new ones" })).toBeNull();
  });
});
