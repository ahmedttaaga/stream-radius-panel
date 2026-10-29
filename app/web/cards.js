// ADV card pages: "مشتركين الكارت" (card series sold to cafés) and "كروت الشحن" (recharge cards).
"use strict";
const timeTxt = (v, unit) => { const n = Number(v || 0); if (!n) return "--"; return `${n} ${({ "1": "الأيام", "2": "ساعات", "3": "دقائق" })[String(unit)] || "ساعات"}`; };
const loadDD = (id, label) => `<span class="dd"><button class="btn-chambray" data-dd="${id}">${label} <i class="fa fa-caret-down"></i></button><div class="dd-menu" id="${id}">
  ${["PDF", "Excel", "إيصالات"].flatMap(k => ["الكل", "نشط", "مستخدَم"].map((s, j) => k === "Excel" ? `<a data-xls="${["", "active", "used"][j]}">${k} — ${s}</a>` : `<a class="soon">${k} — ${s}</a>`)).join("")}</div></span>`;
// "تحميل ← Excel": the series table as CSV (rows tagged data-st="active used"); card codes are never exported from here
function wireExcel(name) {
  document.querySelectorAll("[data-xls]").forEach(a => a.onclick = () => {
    const t = $(".adv-table"), st = a.dataset.xls;
    const head = [...t.querySelectorAll("thead th")].slice(1).map(th => th.textContent.trim());
    const rows = [...t.querySelectorAll("tbody tr[data-st]")].filter(tr => !st || tr.dataset.st.split(" ").includes(st)).map(tr => [...tr.children].slice(1).map(td => td.textContent.trim()));
    downloadCSV(name + (st ? "-" + a.textContent.split("—")[1].trim() : ""), head, rows);
  });
}
function wireDD() {
  document.querySelectorAll("[data-dd]").forEach(b => b.onclick = e => { e.stopPropagation(); const m = $("#" + b.dataset.dd); const o = m.classList.contains("open"); document.querySelectorAll(".dd-menu.open").forEach(x => x.classList.remove("open")); if (!o) m.classList.add("open"); });
  document.querySelectorAll(".dd-menu a.soon, button.soon").forEach(a => a.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
}

PAGES.cardusers = async () => {
  const rows = (await api("cardseries")).map(r => { let x = {}; try { x = JSON.parse(r.raw); } catch (_) {} return x; });
  const profs = await api("profiles/full"), pName = id => (profs.find(p => String(p.adv_id) === String(id)) || {}).name || id;
  const mName = id => (MANAGERS.find(m => String(m.adv_id) === String(id)) || {}).username || "admin";
  const unixDate = t => +t > 1e9 ? advDate(new Date(+t * 1000).toISOString().replace("T", " ").slice(0, 19)) : "";
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-credit-card"></i> مشتركين الكارت</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body"><div class="adv-actions" style="margin:0 0 12px">
      <button class="b-light soon"><i class="fa fa-plus"></i> إنشاء</button><button class="btn-chambray soon">قائمة الكروت</button><button class="btn-red soon">حذف</button>
      <button class="btn-hoki" id="all">عرض الكل</button><button class="btn-plain" id="reload2">تحديث الإحصائيات</button>${loadDD("cLoad", "تحميل")}
      <button class="btn-orange soon">إعادة تفعيل الكروت</button><button class="btn-slate soon">تحقق من</button><button class="b-light soon">تعديل</button>
      <button class="btn-sage soon">المدير</button><button class="btn-lblue soon">استيراد</button></div>
      <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>حزمة</th><th>طريقة الإحتساب</th><th>الباقة</th><th>السعر (EGP)</th>
        <th>السعر (EGP) (كارت)</th><th>عدد الاجهزة</th><th>انشى في</th><th>الكمية</th><th>الوقت</th><th>المدير</th><th>ينتهي في</th><th>صلاحية الكارت بعد اول إتصال</th>
        <th>الكوتة الكلية</th><th>عند انتهاء الكوتة</th><th>مفعل بالماك</th><th>معرف الخدمة</th><th>ملاحظات</th></tr></thead><tbody>
      ${rows.map((s, i) => { const qty = +s.qty || 0, used = +s.usage_cards || 0;
        return `<tr data-st="${[qty - used > 0 ? "active" : "", used > 0 ? "used" : ""].join(" ").trim()}"><td class="c-chk"><input type="checkbox"></td><td>${i + 1}</td><td>${esc(s.name_ser)}</td><td>${+s.at_the_first_login || +s.exp_first_login ? "من اول إتصال" : "من الإنشاء"}</td>
        <td>${esc(pName(s.profile))}</td><td>${fmt(qty * Number(s.price || 0))}</td><td>${Number(s.price || 0).toFixed(2)}</td><td>${esc(s.simul || 1)}</td><td>${esc(unixDate(s.created_on))}</td>
        <td>${qty ? `${qty} <span class="mini ${qty - used > 0 ? "green" : "red"}">${qty - used > 0 ? `متاح ${qty - used}` : "منتهي"}</span>` : `<span class="muted">مستخدم ${used}</span>`}</td>
        <td>${timeTxt(s.val_time_exp || s.time_cards_exp, s.val_date)}</td><td>${esc(mName(s.owner))}</td><td>${+s.exp_date > 1e9 ? esc(unixDate(s.exp_date)) : "--"}</td>
        <td>${timeTxt(s.val_time_exp, s.val_date)}</td><td>${s.all_quota && +s.all_quota ? gbTxt(s.all_quota) : "غير محدود"}</td><td>${+s.exp_quota ? "إيقاف الكارت" : "---"}</td>
        <td>${+s.connect_by_mac || +s.lock_by_mac ? "نعم" : "---"}</td><td>${esc(s.val_serv_name || "---")}</td><td>${esc(s.notes || "")}</td></tr>`; }).join("") || '<tr><td colspan="19" class="empty">لا توجد بيانات</td></tr>'}
      </tbody></table></div><div class="adv-tools"><div class="muted">إظهار 1 إلى ${rows.length} من أصل ${rows.length} مُدخل</div></div></div></div>`);
  $("#reload").onclick = $("#reload2").onclick = $("#all").onclick = () => PAGES.cardusers();
  wireDD(); wireExcel("مشتركين الكارت");
};

PAGES.cards = async () => {
  const r = await api("cards?kind=recharge");
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-credit-card"></i> قائمة الكروت</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body"><div class="adv-actions" style="margin:0 0 12px">
      <button class="b-light" id="gen"><i class="fa fa-plus"></i> إنشاء</button><button class="btn-chambray soon">قائمة الكروت</button><button class="btn-red soon">حذف</button>${loadDD("kLoad", "تحميل")}
      <button class="btn-slate soon">تحقق من</button><button class="btn-sage soon">المدير</button><button class="b-green soon">البيع بالتجزئة</button>
      <button class="btn-blue2 soon">تعديل</button><button class="btn-orange soon">حظر</button><button class="btn-orange soon">تفعيل</button></div>
      <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>حزمة</th><th>السعر (حزمة) (EGP)</th><th>السعر (EGP)</th><th>المدير</th><th>الكمية</th>
        <th>مستخدَم</th><th>انشى في</th><th>انشئ بواسطة</th><th>ينتهي في</th><th>الحالة</th><th>يتم التجديد للمشترك بواسطة</th><th>الباقة</th></tr></thead><tbody>
      ${(() => { const by = {}; r.rows.forEach(c => { const k = c.series || "-"; (by[k] = by[k] || { s: k, n: 0, used: 0, price: c.price, mgr: c.manager, created: c.created, exp: c.expires, prof: c.profile }); by[k].n++; if (c.status === "used") by[k].used++; });
        return Object.values(by).map((b, i) => `<tr data-st="${[b.n - b.used > 0 ? "active" : "", b.used > 0 ? "used" : ""].join(" ").trim()}"><td class="c-chk"><input type="checkbox"></td><td>${i + 1}</td><td>${esc(b.s)}</td><td>${fmt(b.n * b.price)}</td><td>${Number(b.price).toFixed(2)}</td>
          <td>${esc(b.mgr || "admin")}</td><td>${b.n}</td><td>${b.used}</td><td>${esc(advDate(b.created))}</td><td>admin</td><td>${esc(b.exp ? advDate(b.exp) : "--")}</td>
          <td><span class="mini green">نشط</span></td><td>الرصيد</td><td>${esc(b.prof || "")}</td></tr>`).join(""); })() || '<tr><td colspan="14" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}
      </tbody></table></div></div></div>`);
  $("#reload").onclick = () => PAGES.cards();
  $("#gen").onclick = () => {
    const m = modal(`<h3>إنشاء كروت شحن</h3><form class="form" id="gf"><label>العدد<input name="Count" type="number" min="1" max="1000" value="10"></label>
      <label>الباقة<select name="ProfileID">${options(PROFILES, 0)}</select></label><label>السعر<input name="Price" type="number" step="0.01" value="0"></label>
      <label>بادئة<input name="Prefix" dir="ltr"></label><label>طول الكود<input name="Length" type="number" value="10"></label><label>اسم الحزمة<input name="Series"></label>
      <label>المدير<select name="ManagerID">${options(MANAGERS, 0, "username")}</select></label>
      <div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">إنشاء</button></div></form>`);
    $("#gf", m).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target); ["Count", "ProfileID", "Length", "ManagerID", "Price"].forEach(k => f[k] = +f[k]); f.Kind = "recharge";
      const res = await act(() => api("cards/generate", { method: "POST", body: f }));
      modal(`<h3>اتعمل ${res.codes.length} كارت — ${esc(res.series)}</h3><pre>${esc(res.codes.join("\n"))}</pre>`); PAGES.cards();
    };
  };
  wireDD(); wireExcel("كروت الشحن");
};
