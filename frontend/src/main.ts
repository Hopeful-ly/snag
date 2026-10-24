import { mount } from "svelte";
import App from "./App.svelte";
import LinksWindow from "./LinksWindow.svelte";
import PlaylistWindow from "./PlaylistWindow.svelte";
import SaveWindow from "./SaveWindow.svelte";
import { wailsApi } from "./lib/api/client";
import "./lib/styles/tokens.css";
import "./lib/styles/base.css";

const target = document.getElementById("app");
if (!target) throw new Error("missing #app element");

const query = new URLSearchParams(location.search);
const save = query.get("save");
const playlist = query.get("playlist");
const links = query.get("links");
export default save
  ? mount(SaveWindow, { target, props: { api: wailsApi, id: save } })
  : playlist
    ? mount(PlaylistWindow, { target, props: { api: wailsApi, id: playlist } })
    : links
      ? mount(LinksWindow, { target, props: { api: wailsApi, id: links } })
      : mount(App, { target, props: { api: wailsApi } });
