// "NAS" — ADV's NAS page: same buttons and columns. In shadow mode the rows are ADV's NAS list (secrets are never shown).
"use strict";
PAGES.nas = async () => {
  const d = await api("nas");
  const shadow = ME.mode === "shadow";
  const rows = shadow ? (d.adv || []).map(x => ({ ...x, src: "adv" })) : d.local.map(x => ({ id: x.id, name: x.shortname, ip: x.nasname, type: x.type, description: x.description, online: 0, src: "local" }));
  rows.sort((a, b) => String(a.name).localeCompare(String(b.name)));
  const typeAr = t => ({ mikrotik: "مايكروتك", other: "أخرى", cisco: "سيسكو" })[String(t).toLowerCase()] || t || "مايكروتك";
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-paper-plane"></i> NAS</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body">
      ${shadow ? '<div class="alert">وضع المراقبة: دي أجهزة الـ NAS المسجلة في ADV (قراءة بس). الرمز السري مش بيتعرض هنا.</div>' : ""}
      <div class="adv-actions" style="margin:0 0 12px">
        <button class="b-light" id="nAdd"><i class="fa fa-plus"></i> إضافة</button><button class="btn-chambray" id="nEdit">تعديل</button><button class="btn-red" id="nDel">حذف</button>
        <button class="b-light soon">إعادة بناء</button><button class="btn-red soon">إعادة تشغيل</button><button class="b-light soon">DNS</button>
        <span class="dd"><button class="btn-chambray" data-dd="nSvc">خدمات الجهاز <i class="fa fa-caret-down"></i></button><div class="dd-menu" id="nSvc"><a class="soon">هوتسبوت</a><a class="soon">برودباند</a><a class="soon">يوزر مانجر</a></div></span>
        <span class="dd"><button class="btn-chambray" data-dd="nMore">المزيد <i class="fa fa-caret-down"></i></button><div class="dd-menu" id="nMore"><a class="soon">إغلاق كل الجلسات</a><a class="soon">تغيير الحالة</a><a class="soon">إغلاق بالماك ادرس</a><a class="soon">Ping</a><a class="soon">فصل المشتركين</a></div></span>
        <button class="btn-chambray soon">استيراد مشتركين</button><button class="b-light soon">Login Users</button></div>
      <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>الاسم</th><th>عنوان IP</th><th>الرمز السري</th><th>النوع</th><th>الوصف</th><th>المتصلين (مشتركين/كروت)</th><th>SNMP</th></tr></thead>
      <tbody>${rows.map((n, i) => `<tr><td class="c-chk"><input type="checkbox" data-n="${n.src === "local" ? n.id : ""}"></td>
        <td class="c-n" style="background:${+n.online ? "#5ea1f3" : "#2dd22d"}">${esc(n.id || i + 1)}</td><td>${esc(n.name)}</td><td><b>${esc(n.ip)}</b></td><td>••••••</td>
        <td>${esc(typeAr(n.type))}</td><td>${esc(n.description || "")}</td><td>${+n.online || 0}</td><td></td></tr>`).join("") || '<tr><td colspan="9" class="empty">لا توجد بيانات</td></tr>'}</tbody></table></div>
      <div class="adv-tools"><div class="muted">إظهار 1 إلى ${rows.length} من أصل ${rows.length} مُدخل</div></div>
    </div></div>`);
  $("#reload").onclick = () => PAGES.nas();
  $("#nAdd").onclick = () => {
    const m = modal(`<h3>إضافة NAS</h3><form class="form" id="nf"><label>الاسم<input name="Shortname" required></label><label>عنوان IP<input name="Nasname" dir="ltr" required></label>
      <label>الرمز السري<input name="Secret" type="password" required></label><label>النوع<select name="Type"><option value="mikrotik">مايكروتك</option><option value="other">أخرى</option></select></label>
      <label class="full">الوصف<input name="Description"></label><div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
    $("#nf", m).onsubmit = async e => { e.preventDefault(); await act(() => api("nas", { method: "POST", body: formData(e.target) }), "اتضاف"); closeModal(); PAGES.nas(); };
  };
  $("#nEdit").onclick = () => {
    const ids = [...document.querySelectorAll("[data-n]:checked")].map(c => c.dataset.n).filter(Boolean);
    if (ids.length !== 1) return toast(shadow ? "أجهزة ADV ما تتعدلش من هنا (وضع المراقبة)" : "اختار جهاز واحد", true);
    const n = d.local.find(x => String(x.id) === ids[0]);
    const m = modal(`<h3>تعديل NAS</h3><form class="form" id="nf"><label>الاسم<input name="Shortname" value="${esc(n.shortname)}" required></label><label>عنوان IP<input name="Nasname" dir="ltr" value="${esc(n.nasname)}" required></label>
      <label>الرمز السري<input name="Secret" type="password" placeholder="سيبه فاضي لو مش هتغيره" autocomplete="new-password"></label><label>النوع<select name="Type"><option value="mikrotik">مايكروتك</option><option value="other" ${n.type === "other" ? "selected" : ""}>أخرى</option></select></label>
      <label class="full">الوصف<input name="Description" value="${esc(n.description || "")}"></label><div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
    $("#nf", m).onsubmit = async e => { e.preventDefault(); await act(() => api("nas/" + n.id, { method: "PUT", body: formData(e.target) }), "اتعدل"); closeModal(); PAGES.nas(); };
  };
  $("#nDel").onclick = async () => {
    const ids = [...document.querySelectorAll("[data-n]:checked")].map(c => c.dataset.n).filter(Boolean);
    if (!ids.length) return toast(shadow ? "أجهزة ADV ما تتمسحش من هنا" : "اختار جهاز", true);
    if (!confirm(`حذف ${ids.length} جهاز؟`)) return;
    for (const id of ids) await api("nas/" + id, { method: "DELETE" });
    PAGES.nas();
  };
  wireDD();
};
