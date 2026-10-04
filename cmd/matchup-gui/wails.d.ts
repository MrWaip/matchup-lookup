// Globals injected by the Wails runtime. Method types live in frontend/api.js.
import type { GoApp } from "./frontend/api.js";

declare global {
  interface Window {
    go: { gui: { App: GoApp } };
  }
}
