// "مراقبة الباندويث" — ADV's speed control: a 0–1000 % slider per profile (or one for all), ON/OFF per profile.
// Saved as settings bw_pct_<profileId>; the RADIUS rate limit of the profile is scaled by it.
"use strict";
PAGES.bandwidth = async () => {
  const list = await api("profiles/full");
  const S = Object.fromEntries((await api("settings").catch(() => ({ settings: [] }))).settings.map(x => [x.k, x.v]));
  const pct = p => +(S["bw_pct_" + p.id] ?? 100), on = p => (S["bw_on_" + p.id] ?? "1") === "1";
  const spd = p => `${mbps(p.down_speed).replace(" Mbps", "Mb").replace(" Kbps", "Kb")} / ${mbps(p.up_speed).replace(" Mbps", "Mb").replace(" Kbps", "Kb")}`;
  const slider = (name, v) => `<div class="bw-slider"><span>%0</span><input type="range" name="${name}" min="0" max="1000" step="5" value="${v}"><span>%1 000</span>
    <b class="bw-val">%${v}</b><div class="bw-ticks"><i>0</i><i>250</i><i>500</i><i>750</i><i>1 000</i></div></div>`;
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-tachometer"></i> مراقبة الباندويث</span></div><div class="adv-body"><form id="bwf">
    <div class="adv-row"><div class="adv-f"><select id="bwProf"><option value="">كل العروض</option>${list.map(p => `<option value="${p.id}">${esc(p.name)}</option>`).join("")}</select></div>
      <div class="adv-f adv-toggle"><label class="switch"><input type="checkbox" id="bwOnlyOn" checked><span>تطبيق السحب على البروفايلات المفعّلة فقط</span></label></div>
      <div class="adv-actions"><button type="button" class="btn-unified on" id="bwUni">تحكم موحد</button><button type="button" class="btn-plain" id="bwSep">تحميل / رفع منفصل</button></div></div>
    <div class="bw-all"><label>نسبة السرعة لكل البروفايلات %</label>${slider("all", 100)}</div>
    <div class="bw-list">${list.map(p => `<div class="bw-row" data-p="${p.id}"><div class="bw-name"><b>${esc(p.name)}</b><small>${spd(p)}</small></div>
      <label class="onoff"><input type="checkbox" name="on_${p.id}" ${on(p) ? "checked" : ""}><b data-on="ON" data-off="OFF"></b></label>
      <div class="bw-pct"><span>النسبة %</span>${slider("pct_" + p.id, pct(p))}</div></div>`).join("")}</div>
    <div class="form-actions"><button class="b-save">حفظ وتطبيق</button></div></form></div></div>`);
  const f = $("#bwf");
  f.addEventListener("input", e => { if (e.target.type === "range") e.target.parentElement.querySelector(".bw-val").textContent = "%" + e.target.value; });
  f.querySelector("[name=all]").oninput = e => f.querySelectorAll(".bw-row").forEach(r => { const on = r.querySelector("input[type=checkbox]").checked;
    if (!$("#bwOnlyOn").checked || on) { const s = r.querySelector("input[type=range]"); s.value = e.target.value; s.parentElement.querySelector(".bw-val").textContent = "%" + s.value; } });
  $("#bwProf").onchange = e => f.querySelectorAll(".bw-row").forEach(r => r.classList.toggle("hidden", e.target.value && r.dataset.p !== e.target.value));
  $("#bwSep").onclick = () => toast("التحكم المنفصل للتحميل والرفع هيتعمل في المرحلة الجاية", true);
  f.onsubmit = async e => {
    e.preventDefault();
    const body = {};
    list.forEach(p => { body["bw_pct_" + p.id] = f.querySelector(`[name=pct_${p.id}]`).value; body["bw_on_" + p.id] = f.querySelector(`[name=on_${p.id}]`).checked ? "1" : "0"; });
    await act(() => api("settings", { method: "PUT", body }), "اتحفظ واتطبق على الراديوس");
  };
};
