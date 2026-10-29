// "تصميم الكروت": card templates (size, A4 columns × rows, background image, where each field sits, QR code) and
// printing a whole series on A4 with the chosen template. Idea from Haitham's designer — here the template is the one
// actually used when printing, and every card can carry a QR with its code.
"use strict";
const CT_FIELDS = [["code", "رقم الكارت", 50, 48, 18, "#000000", true], ["price", "السعر", 85, 15, 12, "#1e3a8a", true], ["profile", "الباقة", 18, 15, 11, "#1e3a8a", true],
  ["series", "رقم الحزمة", 18, 88, 9, "#475569", true], ["company", "اسم الشركة", 50, 15, 13, "#0f172a", false]];
const CT_DEFAULT = () => ({ id: "t" + Date.now(), name: "قالب جديد", w: 85, h: 54, cols: 3, rows: 7, gap: 2, bg: "", color: "#4f46e5",
  f: Object.fromEntries(CT_FIELDS.map(([k, , x, y, s, c, on]) => [k, { x, y, s, c, on }])), qr: { on: true, x: 85, y: 70, s: 16 } });
function ctLoad() { try { const v = JSON.parse(SETTINGS_CACHE.card_templates || "[]"); return Array.isArray(v) ? v : []; } catch (_) { return []; } }
let SETTINGS_CACHE = {};

// one card as HTML (mm units so the print size is exact)
function cardHTML(t, c, qrSvg) {
  const fld = (k, txt) => { const f = t.f[k]; return f && f.on ? `<div class="ct-f" style="left:${f.x}%;top:${f.y}%;font-size:${f.s}px;color:${esc(f.c)}">${esc(txt)}</div>` : ""; };
  return `<div class="ct-card" style="width:${t.w}mm;height:${t.h}mm;border-color:${esc(t.color)};${t.bg ? `background-image:url('${t.bg}')` : `background:linear-gradient(135deg,${esc(t.color)}22,#fff)`}">
    ${fld("company", c.company)}${fld("profile", c.profile)}${fld("price", c.price ? c.price + " ج" : "")}${fld("code", c.code)}${fld("series", c.series)}
    ${t.qr && t.qr.on && qrSvg ? `<div class="ct-qr" style="left:${t.qr.x}%;top:${t.qr.y}%;width:${t.qr.s}mm;height:${t.qr.s}mm">${qrSvg}</div>` : ""}</div>`;
}
function qrFor(text) {
  if (typeof qrcode !== "function") return "";
  const q = qrcode(0, "M"); q.addData(String(text)); q.make();
  return q.createSvgTag({ cellSize: 2, margin: 0, scalable: true });
}
const CT_SAMPLE = () => ({ code: "4815 1623 42", price: "50", profile: "Speed 10M", series: "20261023-1200", company: SETTINGS_CACHE.company || "Stream-Net" });

PAGES.cardtpl = async () => {
  SETTINGS_CACHE = Object.fromEntries((await api("settings")).settings.map(x => [x.k, x.v]));
  let list = ctLoad(), cur = list[0] ? JSON.parse(JSON.stringify(list[0])) : CT_DEFAULT();
  const cards = await api("cards");
  const series = [...new Set((cards.rows || []).filter(r => r.kind === "recharge" || r.kind === "user").map(r => r.series))].filter(Boolean);
  const draw = () => {
    C(`<div class="dz"><div class="dz-card dz-head"><div class="dz-title"><span class="dz-bub big"><i class="bi bi-palette-fill"></i></span>
        <div><h2>تصميم الكروت والطباعة</h2><small>اعمل قالب، حط كل حاجة في مكانها بالسحب، واطبع أي حزمة كروت على A4.</small></div></div></div>
      <div class="ct-grid"><div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-collection"></i> القوالب</span><button class="b-light" id="ctNew"><i class="bi bi-plus"></i> قالب جديد</button></div>
          ${list.length ? list.map(t => `<a class="ct-item ${t.id === cur.id ? "on" : ""}" data-t="${esc(t.id)}"><i class="bi bi-credit-card-2-front"></i> ${esc(t.name)} <small>${t.cols}×${t.rows}</small></a>`).join("") : '<div class="empty">لسه مفيش قوالب</div>'}
          <h4 style="margin:16px 0 6px">اطبع حزمة</h4>
          <select id="ctSeries">${series.map(s => `<option>${esc(s)}</option>`).join("") || "<option value=''>مفيش حزم كروت متولّدة من هنا لسه</option>"}</select>
          <button class="green" id="ctPrint" style="margin-top:8px;width:100%"><i class="bi bi-printer"></i> اطبع بالقالب ده</button>
          <p class="muted">كروت ADV القديمة أرقامها مش عندنا (ADV مش بيدّيها)، فالطباعة للكروت اللي اتولّدت من هنا.</p></div>
        <form class="dz-card" id="ctForm"><div class="dz-card-h"><span><i class="bi bi-sliders"></i> القالب</span><span>
          <button type="button" class="btn-red" id="ctDel" ${list.find(t => t.id === cur.id) ? "" : "disabled"}>حذف</button> <button class="green">حفظ</button></span></div>
          <div class="g12"><div class="fg s6"><label>الاسم</label><input name="name" value="${esc(cur.name)}"></div><div class="fg s3"><label>اللون</label><input name="color" type="color" value="${esc(cur.color)}"></div>
            <div class="fg s3"><label>الخلفية (صورة لحد 300KB)</label><input type="file" id="ctBg" accept="image/png,image/jpeg,image/webp"></div>
            <div class="fg s3"><label>عرض الكارت (مم)</label><input name="w" type="number" min="30" max="120" value="${cur.w}"></div><div class="fg s3"><label>طول الكارت (مم)</label><input name="h" type="number" min="20" max="90" value="${cur.h}"></div>
            <div class="fg s2"><label>أعمدة</label><input name="cols" type="number" min="1" max="8" value="${cur.cols}"></div><div class="fg s2"><label>صفوف</label><input name="rows" type="number" min="1" max="14" value="${cur.rows}"></div>
            <div class="fg s2"><label>مسافة (مم)</label><input name="gap" type="number" min="0" max="10" step="0.5" value="${cur.gap}"></div></div>
          <div class="ct-fields">${CT_FIELDS.map(([k, t]) => { const f = cur.f[k]; return `<div class="ct-row"><label><input type="checkbox" data-on="${k}" ${f.on ? "checked" : ""}> ${t}</label>
              <input type="number" data-s="${k}" min="6" max="40" value="${f.s}" title="حجم الخط"><input type="color" data-c="${k}" value="${esc(f.c)}" title="اللون"></div>`; }).join("")}
            <div class="ct-row"><label><input type="checkbox" data-on="qr" ${cur.qr.on ? "checked" : ""}> QR برقم الكارت</label><input type="number" data-s="qr" min="8" max="40" value="${cur.qr.s}" title="المقاس بالمم"></div></div>
          <p class="muted">اسحب أي عنصر جوه الكارت علشان تغيّر مكانه. ${cur.bg ? '<a id="ctBgDel">شيل الخلفية</a>' : ""}</p>
          <div class="ct-stage" id="ctStage">${cardHTML(cur, CT_SAMPLE(), qrFor("4815162342"))}</div>
          <p class="muted">صفحة A4 هتشيل ${cur.cols * cur.rows} كارت.</p></form></div></div>`);
    wire();
  };
  const readForm = () => {
    const f = $("#ctForm");
    ["name", "color"].forEach(k => cur[k] = f[k].value);
    ["w", "h", "cols", "rows", "gap"].forEach(k => cur[k] = +f[k].value || cur[k]);
    document.querySelectorAll("[data-on]").forEach(e => { const k = e.dataset.on; (k === "qr" ? cur.qr : cur.f[k]).on = e.checked; });
    document.querySelectorAll("[data-s]").forEach(e => { const k = e.dataset.s; (k === "qr" ? cur.qr : cur.f[k]).s = +e.value; });
    document.querySelectorAll("[data-c]").forEach(e => { cur.f[e.dataset.c].c = e.value; });
  };
  const wire = () => {
    $("#ctForm").oninput = e => { if (e.target.id === "ctBg") return; readForm(); $("#ctStage").innerHTML = cardHTML(cur, CT_SAMPLE(), qrFor("4815162342")); dragWire(); };
    $("#ctBg").onchange = e => {
      const file = e.target.files[0]; if (!file) return;
      if (file.size > 300 * 1024) { toast("الصورة أكبر من 300KB — صغّرها الأول", true); return; }
      const rd = new FileReader(); rd.onload = () => { readForm(); cur.bg = rd.result; draw(); }; rd.readAsDataURL(file);
    };
    if ($("#ctBgDel")) $("#ctBgDel").onclick = () => { readForm(); cur.bg = ""; draw(); };
    $("#ctNew").onclick = () => { cur = CT_DEFAULT(); draw(); };
    document.querySelectorAll("[data-t]").forEach(a => a.onclick = () => { cur = JSON.parse(JSON.stringify(list.find(t => t.id === a.dataset.t))); draw(); });
    $("#ctForm").onsubmit = async e => {
      e.preventDefault(); readForm();
      const i = list.findIndex(t => t.id === cur.id); if (i >= 0) list[i] = cur; else list.push(cur);
      await act(() => api("settings", { method: "PUT", body: { card_templates: JSON.stringify(list) } }), "اتحفظ");
      SETTINGS_CACHE.card_templates = JSON.stringify(list); cur = JSON.parse(JSON.stringify(cur)); draw();
    };
    $("#ctDel").onclick = async () => {
      if (!confirm("حذف القالب «" + cur.name + "»؟")) return;
      list = list.filter(t => t.id !== cur.id);
      await act(() => api("settings", { method: "PUT", body: { card_templates: JSON.stringify(list) } }), "اتحذف");
      SETTINGS_CACHE.card_templates = JSON.stringify(list); cur = list[0] ? JSON.parse(JSON.stringify(list[0])) : CT_DEFAULT(); draw();
    };
    $("#ctPrint").onclick = async () => {
      const s = $("#ctSeries").value; if (!s) return toast("مفيش حزمة", true);
      readForm(); const t = cur;
      const rows = (await api("cards?series=" + encodeURIComponent(s))).rows || [];
      if (!rows.length) return toast("الحزمة فاضية", true);
      const per = t.cols * t.rows, pages = [];
      for (let i = 0; i < rows.length; i += per) pages.push(rows.slice(i, i + per));
      const w = window.open("", "_blank");
      w.document.write(`<!doctype html><html lang="ar" dir="rtl"><head><meta charset="utf-8"><title>${esc(s)}</title><style>${CT_PRINT_CSS}
        .sheet{grid-template-columns:repeat(${t.cols},${t.w}mm);gap:${t.gap}mm}</style></head><body>
        ${pages.map(p => `<div class="sheet">${p.map(c => cardHTML(t, { code: c.code, price: c.price ? fmt(c.price) : "", profile: c.profile || "", series: c.series, company: SETTINGS_CACHE.company || "" }, t.qr.on ? qrFor(c.code) : "")).join("")}</div>`).join("")}
        <script>setTimeout(()=>print(),400)<\/script></body></html>`);
      w.document.close();
    };
    dragWire();
  };
  const dragWire = () => {
    const card = $("#ctStage .ct-card"); if (!card) return;
    card.querySelectorAll(".ct-f, .ct-qr").forEach(el => {
      el.onpointerdown = e => {
        e.preventDefault(); el.setPointerCapture(e.pointerId);
        const key = el.classList.contains("ct-qr") ? "qr" : CT_FIELDS.find(([k]) => { const f = cur.f[k]; return f.on && el.style.left === f.x + "%" && el.style.top === f.y + "%"; })?.[0];
        if (!key) return;
        el.onpointermove = m => {
          const r = card.getBoundingClientRect();
          const x = Math.max(3, Math.min(97, (m.clientX - r.left) / r.width * 100)), y = Math.max(3, Math.min(97, (m.clientY - r.top) / r.height * 100));
          const tgt = key === "qr" ? cur.qr : cur.f[key]; tgt.x = Math.round(x); tgt.y = Math.round(y);
          el.style.left = tgt.x + "%"; el.style.top = tgt.y + "%";
        };
        el.onpointerup = () => { el.onpointermove = null; };
      };
    });
  };
  draw();
};
const CT_PRINT_CSS = `@page{size:A4;margin:6mm}body{margin:0;font-family:Cairo,Tahoma,sans-serif}
  .sheet{display:grid;justify-content:center;page-break-after:always}.sheet:last-child{page-break-after:auto}
  .ct-card{position:relative;overflow:hidden;border:1px dashed #94a3b8;border-radius:2mm;background-size:cover;background-position:center;box-sizing:border-box;-webkit-print-color-adjust:exact;print-color-adjust:exact}
  .ct-f{position:absolute;transform:translate(-50%,-50%);white-space:nowrap;font-weight:800;font-family:Inter,Cairo,monospace}
  .ct-qr{position:absolute;transform:translate(-50%,-50%)}.ct-qr svg{width:100%;height:100%;display:block}`;
