// "التحكم في السرعات": temporary / daily / permanent speed boost for every profile, and restore.
"use strict";
PAGES.speed = async () => {
  const d = await api("speed");
  const on = d.state === "on";
  const modeTxt = { temp: "مؤقت لحد " + (d.until || ""), recurring: `كل يوم من ${d.from} لـ ${d.to}`, permanent: "دايم لحد ما ترجّعه" }[d.mode] || "مقفول";
  const howTxt = d.type === "add" ? `+${d.value} ميجا` : `×${d.value}`;
  C(`<div class="dz">
    <div class="dz-card dz-head"><div class="dz-title"><span class="dz-bub big"><i class="bi bi-lightning-charge-fill"></i></span>
      <div><h2>التحكم في السرعات</h2><small>زوّد سرعة كل الباقات لفترة، أو كل يوم في ساعات معيّنة، وبعدها ترجع لوحدها.</small></div>
      <span class="dz-status ${on ? "ok" : ""}" style="${on ? "" : "background:#f1f5f9;color:#475569;border:1px solid #e2e8f0"}">${on ? `<span class="dz-dot"></span> الزيادة شغالة دلوقتي (${esc(howTxt)})` : "السرعات العادية"}</span></div>
      <p class="muted" style="margin:10px 0 0">الوضع: <b>${esc(modeTxt)}</b>${d.mode && d.mode !== "off" ? ` — ${esc(howTxt)}` : ""}. ${d.live ? "المتصلين بياخدوا السرعة الجديدة على طول من غير ما يفصلوا." : "<b>وضع المراقبة:</b> الإعداد بيتحفظ ويتطبّق على الراديوس الجديد، بس المشتركين لسه على ADV."} ${esc(d.note)}</p>
    </div>
    <div class="dz-kpis">
      ${[["temp", "bi-hourglass-split", "زيادة مؤقتة", `<label>المدة<select name="hours">${[1, 2, 3, 6, 12, 24].map(h => `<option value="${h}">${h} ساعة</option>`).join("")}</select></label>`],
        ["recurring", "bi-clock-history", "كل يوم (ساعات السعادة)", `<label>من<input name="from" type="time" value="${esc(d.from || "02:00")}"></label><label>لـ<input name="to" type="time" value="${esc(d.to || "08:00")}"></label>`],
        ["permanent", "bi-infinity", "دايمة", `<p class="muted">لحد ما تدوس "رجّع السرعات".</p>`]].map(([m, ic, t, extra]) => `
        <form class="dz-card spd" data-mode="${m}"><div class="dz-card-h"><span><i class="bi ${ic}"></i> ${t}</span>${d.mode === m ? '<span class="dz-pill p-green">مختار</span>' : ""}</div>
          <div class="spd-f"><label>الزيادة<select name="how"><option value="multiply:1.5">×1.5</option><option value="multiply:2" selected>×2 (الضعف)</option><option value="multiply:2.5">×2.5</option>
            <option value="multiply:3">×3</option><option value="add:2">+2 ميجا</option><option value="add:5">+5 ميجا</option><option value="add:10">+10 ميجا</option></select></label>${extra}</div>
          <button class="green"><i class="bi bi-lightning-charge"></i> شغّل</button></form>`).join("")}
    </div>
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-sliders"></i> السرعات دلوقتي</span>
      <button class="btn-orange" id="spdOff" ${d.mode === "off" || !d.mode ? "disabled" : ""}><i class="bi bi-arrow-counterclockwise"></i> رجّع السرعات</button></div>
      <div class="tablewrap">${table(d.profiles || [], [["name", "الباقة"], ["users", "مشتركين"], ["base", "السرعة الأساسية", v => `<span dir="ltr">${esc(v || "-")}</span>`],
        ["now", "السرعة دلوقتي", (v, r) => `<span dir="ltr" class="${v !== r.base ? "tag green" : ""}">${esc(v || "-")}</span>`]])}</div></div>
  </div>`);
  document.querySelectorAll("form.spd").forEach(f => f.onsubmit = async e => {
    e.preventDefault();
    const fd = Object.fromEntries(new FormData(f)), [type, value] = fd.how.split(":");
    await act(() => api("speed", { method: "POST", body: { Mode: f.dataset.mode, Type: type, Value: +value, Hours: +fd.hours || 0, From: fd.from || "", To: fd.to || "" } }), "اتحفظ");
    PAGES.speed();
  });
  $("#spdOff").onclick = async () => { await act(() => api("speed", { method: "POST", body: { Mode: "off" } }), "السرعات رجعت عادي"); PAGES.speed(); };
};
