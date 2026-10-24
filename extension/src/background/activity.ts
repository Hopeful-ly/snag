// Snag's download list as it happens, so the toolbar badge can say whether anything downloads.
import type { Download } from "../shared/protocol";
import { action } from "./badge";
import { onMessage, onOpen, send } from "./link";

let downloads: Download[] = [];

function showActivity(): void {
  const active = downloads.some((d) => d.status === "downloading");
  action.setBadgeBackgroundColor({ color: active ? "#2e7d32" : "#1565c0" })?.catch?.(() => {});
}

onOpen(() => {
  send({ type: "subscribe" }).then(
    (reply) => {
      downloads = reply.downloads || [];
      showActivity();
    },
    () => {},
  );
});

onMessage((message) => {
  switch (message.type) {
    case "added":
      downloads = [message.download, ...downloads.filter((d) => d.id !== message.download.id)];
      break;
    case "update": {
      const i = downloads.findIndex((d) => d.id === message.download.id);
      if (i >= 0) downloads[i] = message.download;
      else downloads.unshift(message.download);
      break;
    }
    case "removed":
      downloads = downloads.filter((d) => d.id !== message.id);
      break;
    default:
      return;
  }
  showActivity();
});
