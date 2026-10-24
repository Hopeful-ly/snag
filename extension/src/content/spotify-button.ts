import { spotifyListOf, type SpotifyList } from "../media/spotify";
import { CatalogButton } from "./catalog-button";

const LABELS: Record<SpotifyList, string> = {
  playlist: "Download playlist",
  album: "Download album",
  track: "Download song",
  artist: "Download top songs",
  liked: "Download Liked Songs",
};

export class SpotifyButton extends CatalogButton {
  constructor() {
    super({ host: "snag-spotify", listOf: (url) => spotifyListOf(url), labels: LABELS });
  }
}
