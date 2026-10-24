import { appleListOf, type AppleList } from "../media/apple";
import { CatalogButton } from "./catalog-button";

const LABELS: Record<AppleList, string> = {
  playlist: "Download playlist",
  album: "Download album",
  song: "Download song",
  artist: "Download top songs",
};

export class AppleButton extends CatalogButton {
  constructor() {
    super({ host: "snag-apple", listOf: (url) => appleListOf(url), labels: LABELS });
  }
}
