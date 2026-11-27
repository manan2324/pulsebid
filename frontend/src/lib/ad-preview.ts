import sanitizeHtml from "sanitize-html";
import { campaignCopy as c } from "./copy/campaign";
export function neutralizeMarkup(markup: string) {
  const replaced = markup
    .replace(/\{click_url\}/gi, "#")
    .replace(/\{impression_url\}/gi, "about:blank")
    .replace(/https?:\/\/[^\s"'<>]*\/v1\/events\/[^\s"'<>]*/gi, "about:blank");
  return sanitizeHtml(replaced, {
    allowedTags: [
      "div",
      "span",
      "p",
      "a",
      "img",
      "strong",
      "em",
      "b",
      "i",
      "u",
      "br",
      "hr",
      "h1",
      "h2",
      "h3",
      "ul",
      "ol",
      "li",
      "table",
      "tbody",
      "tr",
      "td",
      "th",
    ],
    allowedAttributes: {
      "*": ["style"],
      a: ["href", "title"],
      img: ["src", "alt", "width", "height"],
    },
    allowedSchemes: ["data"],
    allowedSchemesByTag: { img: ["data"] },
    transformTags: {
      a: (_tag, attrs) => ({ tagName: "a", attribs: { ...attrs, href: "#" } }),
      img: (_tag, attrs) => ({
        tagName: "img",
        attribs: {
          ...attrs,
          src: /^data:image\/(png|gif|jpeg|webp);base64,/i.test(attrs.src ?? "")
            ? attrs.src!
            : "",
          alt: attrs.alt ?? c.externalImage,
        },
      }),
    },
    allowedStyles: {
      "*": {
        color: [/^[#\w(),.%\s-]+$/],
        "background-color": [/^[#\w(),.%\s-]+$/],
        "font-size": [/^\d+(px|em|rem|%)$/],
        "font-family": [/^[\w\s,'-]+$/],
        "text-align": [/^(left|right|center)$/],
        padding: [/^[\d.\s]+(px|em|rem|%)?$/],
        margin: [/^[\d.\s]+(px|em|rem|%)?$/],
        display: [/^(block|inline|inline-block|flex)$/],
        width: [/^\d+(px|%)$/],
        height: [/^\d+(px|%)$/],
      },
    },
  });
}
export function previewDocument(markup: string) {
  return `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><meta name="referrer" content="no-referrer"></head><body style="margin:0">${neutralizeMarkup(markup)}</body></html>`;
}
