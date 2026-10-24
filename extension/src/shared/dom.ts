// Element builders for the in-page UI. Nothing is ever set as HTML: pages can be hostile.

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text != null) e.textContent = text;
  return e;
}

/** The tooltip doubles as the accessible name when the button shows no text. */
export function button(
  className: string,
  title: string,
  ...children: (Node | string)[]
): HTMLButtonElement {
  const b = el("button", className);
  b.title = title;
  if (!children.some((c) => typeof c === "string")) b.setAttribute("aria-label", title);
  b.append(...children);
  return b;
}

const SVG = "http://www.w3.org/2000/svg";

/** An icon: [element, attributes] pairs drawn in a viewBox, in the current text color. */
export type Icon = {
  viewBox: string;
  style: Record<string, string>;
  shapes: [string, Record<string, string>][];
};

export function icon(spec: Icon): SVGSVGElement {
  const svg = document.createElementNS(SVG, "svg");
  svg.setAttribute("viewBox", spec.viewBox);
  for (const [name, value] of Object.entries(spec.style)) svg.setAttribute(name, value);
  for (const [tag, attributes] of spec.shapes) {
    const shape = document.createElementNS(SVG, tag);
    for (const [name, value] of Object.entries(attributes)) shape.setAttribute(name, value);
    svg.append(shape);
  }
  return svg;
}

export function withChildren<E extends HTMLElement>(parent: E, ...children: (Node | string)[]): E {
  parent.append(...children);
  return parent;
}

/** A full-page layer above everything that lets clicks through where it shows nothing. */
export function pageLayer(tag: string): HTMLElement {
  const host = document.createElement(tag);
  for (const [k, v] of Object.entries({
    // inherit: all: initial on a node under <html> makes Chromium report the page as light,
    // which overrides Vivaldi's Website Appearance.
    "color-scheme": "inherit",
    position: "fixed",
    top: "0",
    left: "0",
    width: "0",
    height: "0",
    "z-index": "2147483647",
    display: "block",
    "pointer-events": "none",
  })) {
    host.style.setProperty(k, v, "important");
  }
  return host;
}
