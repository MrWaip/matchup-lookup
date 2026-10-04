import { champions } from "./api.js";
import { h, showError } from "./dom.js";

/** @import { Champion } from "./api.js" */

const ANY = { id: "", name: "Any champion" };

/**
 * Turns an input with a sibling listbox into a fuzzy champion picker. The
 * input keeps the chosen champion's name ("" for any); typing shows the best
 * matches from Go's fuzzy ranking. onChange fires after a pick or clear.
 * @param {HTMLInputElement} input
 * @param {HTMLUListElement} list
 * @param {() => void} onChange
 */
export function championCombobox(input, list, onChange) {
  /** @type {Champion[]} */
  let options = [];
  let active = 0;
  let committed = input.value;
  let request = 0;

  const open = () => {
    list.hidden = false;
    input.setAttribute("aria-expanded", "true");
  };

  const close = () => {
    list.hidden = true;
    input.setAttribute("aria-expanded", "false");
    input.removeAttribute("aria-activedescendant");
  };

  const render = () => {
    list.replaceChildren(...options.map((champion, index) => {
      const option = h("li", {
        id: `${list.id}-${index}`,
        role: "option",
        className: champion === ANY ? "any" : "",
        onmousedown: (event) => {
          event.preventDefault();
          pick(champion);
        },
      }, champion.name);
      option.setAttribute("aria-selected", String(index === active));
      return option;
    }));
    if (options.length === 0) list.append(h("li", { className: "none" }, "No champion found"));
    input.setAttribute("aria-activedescendant", `${list.id}-${active}`);
    list.children[active]?.scrollIntoView({ block: "nearest" });
  };

  const suggest = async () => {
    const query = input.value.trim();
    const current = ++request;
    try {
      const found = await champions(query, 12);
      if (current !== request) return;
      options = query === "" ? [ANY, ...found] : found;
      active = 0;
      render();
      open();
    } catch (error) {
      showError(error);
    }
  };

  /** @param {Champion} champion */
  const pick = (champion) => {
    input.value = champion.id === "" ? "" : champion.name;
    close();
    if (input.value !== committed) {
      committed = input.value;
      onChange();
    }
  };

  input.addEventListener("focus", () => {
    input.select();
    suggest();
  });
  input.addEventListener("input", suggest);
  input.addEventListener("blur", () => {
    // Leaving the field keeps the last valid pick; typed text alone is not a filter.
    input.value = committed;
    close();
  });
  input.addEventListener("keydown", (event) => {
    if (list.hidden && event.key !== "Escape") return;
    switch (event.key) {
      case "ArrowDown":
      case "ArrowUp":
        event.preventDefault();
        if (options.length > 0) {
          active = (active + (event.key === "ArrowDown" ? 1 : -1) + options.length) % options.length;
          render();
        }
        break;
      case "Enter": {
        event.preventDefault();
        const champion = options[active];
        if (champion) pick(champion);
        break;
      }
      case "Escape":
        input.value = committed;
        close();
        input.blur();
        break;
    }
  });

  return {
    /** Sets the value without firing onChange, e.g. when restoring filters. */
    set(/** @type {string} */ value) {
      input.value = committed = value;
    },
  };
}
