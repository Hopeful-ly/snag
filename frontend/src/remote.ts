import { mount } from "svelte";
import RemoteApp from "./RemoteApp.svelte";
import { httpApi } from "./lib/remote/api";
import "./lib/styles/tokens.css";

const target = document.getElementById("app");
if (!target) throw new Error("missing #app element");

export default mount(RemoteApp, { target, props: { api: httpApi } });
