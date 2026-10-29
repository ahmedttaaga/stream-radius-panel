// Smaller ADV pages: overviews ("نظرة عامة"), IP Pool, prices, smart features, tools.
"use strict";
const viewOf = () => new URLSearchParams(location.hash.split("?")[1] || "").get("view");
const statBox = (cls, num, lbl, ico, link) => `<div class="small-box ${cls}"><div class="inner"><h3>${num}</h3><p>${lbl}</p></div><div class="icon"><i class="fa ${ico}"></i></div>
  <a class="small-box-footer" href="#${link || ""}">تفاصيل أخرى <i class="fa fa-arrow-circle-left"></i></a></div>`;

// overview pages wrap the list pages
for (const [k, fn] of Object.entries({
  subscribers: async () => {
    const d = await api("dashboard");
    const byProf = await api("profiles/full");
    C(`<div class="row3">${statBox("bg-blue-steel", fmt(d.subscribers), "مشتركين", "fa-users", "subscribers")}${statBox("bg-green", fmt(d.active), "المشتركين المتاحين", "fa-smile-o", "subscribers?status=active")}
      ${statBox("bg-yellow-active", fmt(d.expired), "المشتركين المنتهي اشتراكهم", "fa-frown-o", "subscribers?status=expired")}</div>
      <div class="row3">${statBox("bg-blue-steel", fmt(d.mode === "shadow" ? d.adv_online : d.online), "المشتركين المتصلين", "fa-lightbulb-o", "online")}
      ${statBox("bg-yellow", fmt(d.expiring), "ينتهي اشتراكهم خلال 3 ايام", "fa-meh-o", "subscribers?status=expiring")}${statBox("bg-red", fmt(d.disabled), "الموقوفين", "fa-ban", "subscribers?status=disabled")}</div>
      <div class="adv-card"><div class="adv-head"><span>المشتركين حسب العرض</span></div><div class="adv-body"><div class="tablewrap">${table(byProf.filter(p => p.users > 0).sort((a, b) => b.users - a.users),
        [["name", "العرض"], ["price", "السعر"], ["users", "عدد المشتركين"]])}</div></div></div>`);
  },
  profiles: async () => {
    const list = await api("profiles/full"), used = list.filter(p => p.users > 0);
    C(`<div class="row3">${statBox("bg-blue-steel", list.length, "العروض", "fa-gift", "profiles")}${statBox("bg-green", used.length, "عروض عليها مشتركين", "fa-check", "profiles")}
      ${statBox("bg-g_money", fmt(used.reduce((a, p) => a + p.users * p.price, 0)) + " EGP", "قيمة الاشتراكات الشهرية تقريباً", "fa-money", "profiles")}</div>
      <div class="adv-card"><div class="adv-body"><div class="tablewrap">${table(used.sort((a, b) => b.users - a.users), [["name", "العرض"], ["price", "السعر"], ["users", "المشتركين"]])}</div></div></div>`);
  },
  managers: async () => {
    const list = await api("managers");
    C(`<div class="row3">${statBox("bg-blue-steel", list.length, "المدراء", "fa-university", "managers")}${statBox("bg-green", fmt(list.reduce((a, m) => a + Number(m.balance || 0), 0)), "إجمالي أرصدة المدراء", "fa-money", "managers")}
      ${statBox("bg-g_money", fmt(list.reduce((a, m) => a + Number(m.users || 0), 0)), "مشتركين المدراء", "fa-users", "managers")}</div>
      <div class="adv-card"><div class="adv-body"><div class="tablewrap">${table(list.sort((a, b) => b.users - a.users), [["username", "المدير"], ["full_name", "الاسم"], ["balance", "الرصيد"], ["users", "المشتركين"]])}</div></div></div>`);
  },
  tickets: async () => { C(`<div class="row3">${statBox("bg-blue-steel", "—", "الشكاوى", "fa-exclamation-circle", "tickets")}</div>`); },
})) {
  const orig = PAGES[k];
  PAGES[k] = () => viewOf() === "overview" ? fn() : orig();
}
// "إضافة مدير" opens the list with the add form
{ const orig = PAGES.managers; PAGES.managers = async () => { await orig(); if (new URLSearchParams(location.hash.split("?")[1] || "").get("add") === "1") document.querySelector('[data-mact="add"]')?.click(); }; }

PAGES.ippool = async () => {
  const list = await api("profiles/full"), pools = {};
  list.forEach(p => { const n = p.pool || (p.f || {}).pool_name; if (n) (pools[n] = pools[n] || []).push(p.name); });
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-sitemap"></i> IP Pool</span></div><div class="adv-body">
    <p class="muted">الـ Pools نفسها متعرّفة على السيرفرات (MikroTik). هنا اسم كل Pool والعروض اللي بتستخدمه.</p>
    <div class="tablewrap">${table(Object.entries(pools).map(([n, ps]) => ({ n, c: ps.length, ps: ps.join("، ") })), [["n", "اسم الـ Pool"], ["c", "عدد العروض"], ["ps", "العروض"]])}</div></div></div>`);
};
PAGES.pricing = async () => {
  const list = await api("profiles/full");
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-tags"></i> اسعار العروض</span></div><div class="adv-body">
    <div class="tablewrap">${table(list, [["name", "العرض"], ["price", "السعر الأساسي (EGP)", v => Number(v).toFixed(2)], ["users", "المشتركين"]])}</div>
    <p class="muted">الأسعار الخاصة لكل مدير هتتعمل في المرحلة الجاية.</p></div></div>`);
};
PAGES.smart = async () => {
  const d = await api("settings").catch(() => ({ settings: [] })), S = Object.fromEntries((d.settings || []).map(x => [x.k, x.v]));
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-magic"></i> المزايا الذكية</span></div><div class="adv-body"><form id="lf">
    <h3 class="form-section">نقاط تشجيعية <div class="adv-sw"><span>مفعّلة</span><label class="onoff"><input type="checkbox" name="loyalty_on" ${S.loyalty_on === "1" ? "checked" : ""}><b data-on="ON" data-off="OFF"></b></label></div></h3>
    <div class="g12"><div class="fg s6"><label>خيارات</label><select name="loyalty_for"><option value="manager" ${S.loyalty_for === "manager" ? "selected" : ""}>للمدير</option>
      <option value="subscriber" ${S.loyalty_for !== "manager" ? "selected" : ""}>للمشترك</option></select></div></div>
    <p class="muted"><b>كيف تعمل؟</b> يكسب المشترك نقاط عند كل تجديد أو شحن (1 نقطة لكل 10 وحدة عملة). يقدر يستبدل 100 نقطة برصيد. النقاط بتتحسب تلقائي.</p>
    <div class="form-actions"><button class="b-save">حفظ</button></div></form></div></div>`);
  $("#lf").onsubmit = async e => { e.preventDefault(); const f = formData(e.target); f.loyalty_on = e.target.loyalty_on.checked ? "1" : "0"; await act(() => api("settings", { method: "PUT", body: f }), "تم الحفظ"); };
};
PAGES.tools = () => PAGES.sync();
