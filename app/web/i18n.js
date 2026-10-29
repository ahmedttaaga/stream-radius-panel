// Language switch (عربي / English). The panel is written in Arabic; in English every text shown on screen is
// translated from I18N_EN (i18n-en.js) as it appears — pages, modals, toasts, server messages. The sidebar stays on
// the RIGHT in both languages (owner's choice). Data typed by people (names, notes) is only touched on an exact match.
"use strict";
const LANG = (() => { try { return localStorage.getItem("rm_lang") || "ar"; } catch (_) { return "ar"; } })();
const I18N = LANG === "en" && typeof I18N_EN === "object" ? I18N_EN : null;
// phrases used inside longer texts (labels around numbers): longest first, 3+ letters, not single short words
const I18N_PHRASES = I18N ? Object.keys(I18N).filter(k => k.length >= 6 || (k.length >= 4 && k.includes(" "))).sort((a, b) => b.length - a.length) : [];
const HAS_AR = /[؀-ۿ]/;

function tr(s) {
  if (!I18N || s == null) return s;
  const str = String(s);
  if (!HAS_AR.test(str)) return str;
  const lead = str.match(/^\s*/)[0], tail = str.match(/\s*$/)[0], core = str.trim();
  if (I18N[core] !== undefined) return lead + I18N[core] + tail;
  let out = core;
  for (const k of I18N_PHRASES) {
    if (out.includes(k)) out = out.split(k).join(I18N[k]);
    if (!HAS_AR.test(out)) break;
  }
  // labels around numbers ("3 أيام"): single words too — only when a digit is there, so people's names stay as typed
  if (HAS_AR.test(out) && /[0-9]/.test(out)) out = out.split(/(\s+)/).map(w => (I18N[w] !== undefined ? I18N[w] : w)).join("");
  return lead + out + tail;
}

function trTree(root) {
  if (!I18N || !root) return;
  if (root.nodeType === 3) { const v = tr(root.nodeValue); if (v !== root.nodeValue) root.nodeValue = v; return; }
  if (root.nodeType !== 1) return;
  const skip = n => n.closest && n.closest("script,style,textarea,[data-noi18n]");
  const w = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const nodes = [];
  for (let n = w.nextNode(); n; n = w.nextNode()) if (HAS_AR.test(n.nodeValue) && !skip(n.parentElement)) nodes.push(n);
  nodes.forEach(n => { const v = tr(n.nodeValue); if (v !== n.nodeValue) n.nodeValue = v; });
  const els = [root, ...root.querySelectorAll("[placeholder],[title],[aria-label],input[type=button],input[type=submit],option")];
  els.forEach(e => {
    ["placeholder", "title", "aria-label"].forEach(a => { const v = e.getAttribute && e.getAttribute(a); if (v && HAS_AR.test(v)) e.setAttribute(a, tr(v)); });
    if ((e.type === "button" || e.type === "submit") && HAS_AR.test(e.value)) e.value = tr(e.value);
  });
}

(function initLang() {
  const html = document.documentElement;
  html.lang = LANG === "en" ? "en" : "ar";
  html.dir = LANG === "en" ? "ltr" : "rtl";
  html.classList.toggle("lang-en", LANG === "en");
  if (!I18N) return;
  const _alert = window.alert.bind(window), _confirm = window.confirm.bind(window), _prompt = window.prompt.bind(window);
  window.alert = m => _alert(tr(m));
  window.confirm = m => _confirm(tr(m));
  window.prompt = (m, d) => _prompt(tr(m), d);
  document.title = tr(document.title);
  let busy = false;
  const obs = new MutationObserver(list => {
    if (busy) return;
    busy = true;
    try {
      for (const m of list) {
        if (m.type === "characterData") { if (HAS_AR.test(m.target.nodeValue)) trTree(m.target); continue; }
        m.addedNodes.forEach(n => trTree(n));
      }
    } finally { busy = false; }
  });
  const start = () => { trTree(document.body); obs.observe(document.body, { childList: true, subtree: true, characterData: true }); };
  if (document.body) start(); else document.addEventListener("DOMContentLoaded", start);
})();

function setLang(l) { try { localStorage.setItem("rm_lang", l); } catch (_) {} location.reload(); }
document.addEventListener("DOMContentLoaded", () => { const b = document.getElementById("langBtn"); if (b) b.textContent = LANG === "en" ? "عربي" : "EN"; });
