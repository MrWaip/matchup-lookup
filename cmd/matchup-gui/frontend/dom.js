/**
 * Returns the element matching selector, typed by its expected class.
 * @template {Element} T
 * @param {string} selector
 * @param {new () => T} type
 * @returns {T}
 */
export function $(selector, type) {
  const element = document.querySelector(selector);
  if (!(element instanceof type)) throw new Error(`${selector} is not a ${type.name}`);
  return element;
}

/**
 * Creates an element. Children are appended as text or nodes, so values
 * such as Riot IDs are never parsed as HTML.
 * @template {keyof HTMLElementTagNameMap} K
 * @param {K} tag
 * @param {Partial<HTMLElementTagNameMap[K]>} [props]
 * @param {...(Node | string)} children
 * @returns {HTMLElementTagNameMap[K]}
 */
export function h(tag, props = {}, ...children) {
  const element = Object.assign(document.createElement(tag), props);
  element.append(...children);
  return element;
}

/** @param {string} message @param {"info" | "error"} [kind] */
export function toast(message, kind = "info") {
  const item = h("div", { className: `toast ${kind}`, role: kind === "error" ? "alert" : "status" }, message);
  $("#toasts", HTMLElement).append(item);
  setTimeout(() => item.remove(), kind === "error" ? 8000 : 4000);
}

/** @param {unknown} error */
export const showError = (error) => toast(String(error), "error");
