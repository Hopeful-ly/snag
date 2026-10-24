import { describe, expect, it } from "vitest";
import { checksumFiles, hashesIn, pageChecksums } from "../src/media/checksum";

const sha256 = "3f0b".repeat(16);
const other = "9c1e".repeat(16);
const md5 = "d41d8cd98f00b204e9800998ecf8427e";

function page(html: string, base = "https://example.org/downloads/"): Document {
  const doc = document.implementation.createHTMLDocument("t");
  const b = doc.createElement("base");
  b.href = base;
  doc.head.append(b);
  doc.body.innerHTML = html;
  return doc;
}

describe("hashes in text", () => {
  it("finds checksums with their names, and nothing that isn't one", () => {
    expect(hashesIn(`SHA256: ${sha256}`)).toEqual([`SHA256: ${sha256}`]);
    expect(hashesIn(`md5sum ${md5}`)).toEqual([`MD5: ${md5}`]);
    expect(hashesIn(`checksum ${sha256.toUpperCase()}.`)).toEqual([sha256]);
    expect(hashesIn("12345678901234567890123456789012")).toEqual([]); // digits alone
    expect(hashesIn(`x${sha256}y`)).toEqual([]); // inside a longer word
    expect(hashesIn(`/files/${sha256}/tool.zip`, `https://e.org/files/${sha256}/tool.zip`)).toEqual(
      [],
    );
  });
});

describe("a page's checksums", () => {
  it("reads the hash in the link's table row", () => {
    const doc = page(`<table>
      <tr><td><a href="tool-1.0.zip">tool-1.0.zip</a></td><td>12 MB</td><td><code>${sha256}</code></td></tr>
      <tr><td><a href="tool-1.0.tar.gz">tool-1.0.tar.gz</a></td><td>11 MB</td><td><code>${other}</code></td></tr>
    </table>`);
    expect(pageChecksums(doc, "https://example.org/downloads/tool-1.0.zip")).toEqual({
      checksum: sha256,
      files: [],
    });
    expect(pageChecksums(doc, "https://example.org/downloads/tool-1.0.tar.gz")?.checksum).toBe(
      other,
    );
  });

  it("reads the hash under the link, named", () => {
    const doc = page(`<div class="release">
        <p><a href="https://cdn.example.org/distro.iso">Download distro.iso</a></p>
        <p class="small">SHA-256 checksum: ${sha256}</p>
      </div>
      <div class="release"><p><a href="old.iso">old</a></p><p>SHA-256: ${other}</p></div>`);
    expect(pageChecksums(doc, "https://cdn.example.org/distro.iso")?.checksum).toBe(
      `SHA256: ${sha256}`,
    );
  });

  it("takes the strongest of a file's checksums, and none when it can't tell whose", () => {
    const doc = page(`<ul>
      <li><a href="a.exe">a.exe</a> MD5 ${md5} SHA256 ${sha256}</li>
    </ul><div><a href="b.exe">b.exe</a><span>${sha256} ${other}</span></div>`);
    expect(pageChecksums(doc, "https://example.org/downloads/a.exe")?.checksum).toBe(
      `SHA256: ${sha256}`,
    );
    expect(pageChecksums(doc, "https://example.org/downloads/b.exe")?.checksum).toBeUndefined();
  });

  it("lists checksum files, the file's own first", () => {
    const doc = page(`
      <a href="SHA256SUMS">SHA256SUMS</a>
      <a href="SHA256SUMS.gpg">signature</a>
      <a href="distro.iso">distro.iso</a>
      <a href="distro.iso.sha256">distro.iso.sha256</a>
      <a href="notes.txt">notes</a>`);
    expect(checksumFiles(doc, "https://example.org/downloads/distro.iso")).toEqual([
      "https://example.org/downloads/distro.iso.sha256",
      "https://example.org/downloads/SHA256SUMS",
    ]);
    expect(pageChecksums(doc, "https://example.org/downloads/distro.iso")).toEqual({
      files: [
        "https://example.org/downloads/distro.iso.sha256",
        "https://example.org/downloads/SHA256SUMS",
      ],
    });
  });

  it("says nothing for a page without the link or checksum files", () => {
    expect(pageChecksums(page(`<p>${sha256}</p>`), "https://example.org/x.zip")).toBeNull();
  });

  it("doesn't read a whole page's hashes as one file's", () => {
    const rows = Array.from({ length: 80 }, (_, i) => `<p>file-${i}.zip ${other}</p>`).join("");
    const doc = page(`<div><div><div><a href="mine.zip">mine</a></div></div>${rows}</div>`);
    expect(pageChecksums(doc, "https://example.org/downloads/mine.zip")?.checksum).toBeUndefined();
  });
});
