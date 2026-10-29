// "الادوات" — ADV's tools: backups, trash, maintenance, general adjustments.
"use strict";
const sizeTxt = b => b > 1048576 ? (b / 1048576).toFixed(2) + " MB" : (b / 1024).toFixed(2) + " KB";
PAGES.backup = async () => {
  const rows = await api("backups");
  const sel = new Set();
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-database"></i> نسخ إحتياطية & استرجاعها <small>(تم إيجاد ${rows.length} نتائج )</small></span></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="b-light" id="bkNew">إنشاء</button><button class="btn-chambray" id="bkDl">تحميل</button><button class="b-light soon">نسخ رابط النسخة</button>
      <button class="b-light" id="bkUp">رفع</button><button class="b-light" id="bkRestore">استرجاع</button><input type="file" id="bkFile" accept=".gz" hidden><button class="b-light soon">استعادة البيانات المحددة</button><button class="b-light soon">تنزيل من خادم خارجي</button><button class="btn-red" id="bkDel">حذف</button></div>
    <div class="adv-tools"><label>إظهار <select>${[10, 25, 50, 100].map(n => `<option>${n} في الصفحة</option>`).join("")}</select> نتائج</label><label>بحث <input id="bkq"></label></div>
    <div class="tablewrap"><table class="adv-table" id="bkT"></table></div>
    <p class="muted">نسخة تلقائية كل يوم 3:30 الفجر، وبتتحفظ 14 يوم. الاسترجاع بيتعمل من السيرفر نفسه عشان الأمان.</p></div></div>`);
  const draw = () => { const q = $("#bkq").value.trim();
    $("#bkT").innerHTML = `<thead><tr><th class="c-chk"><input type="checkbox" id="bkAll"></th><th>#</th><th>اسم الملف</th><th>التاريخ</th><th>الحجم</th></tr></thead><tbody>${rows.filter(x => !q || x.name.includes(q)).map((x, i) =>
      `<tr><td class="c-chk"><input type="checkbox" data-b="${esc(x.name)}" ${sel.has(x.name) ? "checked" : ""}></td><td>${i + 1}</td><td dir="ltr">${esc(x.name)}</td><td>${advDate(x.date)}</td><td>${sizeTxt(x.size)}</td></tr>`).join("") || '<tr><td colspan="5" class="empty">لا توجد نسخ</td></tr>'}</tbody>`;
    document.querySelectorAll("[data-b]").forEach(c => c.onchange = () => c.checked ? sel.add(c.dataset.b) : sel.delete(c.dataset.b));
    $("#bkAll").onchange = e => { rows.forEach(x => e.target.checked ? sel.add(x.name) : sel.delete(x.name)); draw(); }; };
  $("#bkq").oninput = draw;
  $("#bkNew").onclick = async () => { const r = await act(() => api("backups/create", { method: "POST" }), "اتعملت النسخة"); PAGES.backup(); };
  $("#bkDl").onclick = () => { if (!sel.size) return toast("اختار نسخة", true); [...sel].forEach(n => window.open("/api/backups/download?name=" + encodeURIComponent(n), "_blank")); };
  $("#bkRestore").onclick = () => {
    if (sel.size !== 1) return toast("اختار نسخة واحدة", true);
    const name = [...sel][0];
    const m = modal(`<h3 class="red"><i class="fa fa-warning"></i> استرجاع نسخة</h3><p>الاسترجاع <b>هيمسح كل البيانات الحالية</b> ويرجّع النسخة: <b dir="ltr">${esc(name)}</b></p>
      <p class="muted">خد نسخة جديدة الأول ("إنشاء") لو مش متأكد.</p><form class="form" id="rsf"><label class="full">علشان تأكد اكتب: نعم استرجع<input name="Confirm" autocomplete="off"></label>
      <div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="btn-red">استرجاع</button></div></form>`);
    $("#rsf", m).onsubmit = async e => { e.preventDefault(); await act(() => api("backups/restore", { method: "POST", body: { Name: name, Confirm: formData(e.target).Confirm } }), "اترجعت النسخة"); closeModal(); PAGES.backup(); };
  };
  $("#bkUp").onclick = () => $("#bkFile").click();
  $("#bkFile").onchange = async e => {
    const f = e.target.files[0]; if (!f) return;
    const fd = new FormData(); fd.append("file", f);
    const res = await fetch("/api/backups/upload", { method: "POST", body: fd, headers: { "X-Requested-With": "rm" }, credentials: "same-origin" });
    const j = await res.json().catch(() => ({}));
    if (!res.ok) return toast(j.error || "فشل الرفع", true);
    toast("اترفعت النسخة"); PAGES.backup();
  };
  $("#bkDel").onclick = async () => { if (!sel.size) return toast("اختار نسخة", true); if (confirm(`حذف ${sel.size} نسخة؟`)) { await act(() => api("backups", { method: "DELETE", body: { Names: [...sel] } }), "اتحذفت"); PAGES.backup(); } };
  document.querySelectorAll(".soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
  draw();
};

PAGES.trash = async () => {
  const rows = await api("trash");
  const sel = new Set();
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-trash"></i> سلة المحذوفات</span></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="b-green" data-ta="restore">استعادة المحدد</button><button class="btn-orange" data-ta="purge">حذف المحدد</button><button class="btn-red" id="trEmpty">إفراغ السلة</button>
      <span class="spacer"></span><button class="btn-lblue">الكل</button>${["مشتركين", "حزم كروت", "كروت", "مديرين", "عروض", "NAS", "باك اب"].map(t => `<button class="btn-plain">${t}</button>`).join("")}</div>
    <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>الاسم</th><th>النوع</th><th>معلومات إضافية</th><th>حذف بواسطة</th><th>تاريخ الحذف</th><th>الحجم</th><th>إجراءات</th></tr></thead>
      <tbody>${rows.map((x, i) => `<tr><td class="c-chk"><input type="checkbox" data-tr="${x.id}"></td><td>${i + 1}</td><td>${esc(x.name)}</td><td>${esc(x.kind)}</td><td>${esc(x.info)}</td><td>${esc(x.deleted_by || "")}</td><td>${advDate(x.deleted_at)}</td><td>—</td>
        <td><button class="b-green sm" data-one="restore" data-id="${x.id}">استعادة</button> <button class="btn-red sm" data-one="purge" data-id="${x.id}">حذف</button></td></tr>`).join("") || '<tr><td colspan="9" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}</tbody></table></div></div></div>`);
  document.querySelectorAll("[data-tr]").forEach(c => c.onchange = () => c.checked ? sel.add(+c.dataset.tr) : sel.delete(+c.dataset.tr));
  const run = async (action, ids) => { if (!ids.length) return toast("اختار عناصر", true); if (action === "purge" && !confirm("حذف نهائي؟ مش هينفع يرجع")) return; await act(() => api("trash", { method: "POST", body: { Action: action, IDs: ids } }), "تم"); PAGES.trash(); };
  document.querySelectorAll("[data-ta]").forEach(b => b.onclick = () => run(b.dataset.ta, [...sel]));
  document.querySelectorAll("[data-one]").forEach(b => b.onclick = () => run(b.dataset.one, [+b.dataset.id]));
  $("#trEmpty").onclick = () => run("purge", rows.map(x => x.id));
};

PAGES.maintenance = async () => {
  const m = await api("maintenance");
  const st = on => on ? '<span class="st-on">يعمل</span>' : '<span class="st-off">ﻻ يعمل</span>';
  const sec = (title, body, extra = "") => `<div class="mt-sec"><h4>${title}</h4>${body}<div class="adv-actions">${extra}</div></div>`;
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-wrench"></i> صيانة النظام</span></div><div class="adv-body mt-grid">
    ${sec("تحديث النظام", "<p>يقوم هذا الزر بالاستعلام عن ما اذا كان هناك تحديثات جديدة ام لا</p>", '<button class="b-light soon">تحديث الإصدار</button>')}
    ${sec("RADIUS Server", `<p>الحالة : ${st(m.radius)}</p>`, '<button class="b-light" data-m="radius_start">تشغيل</button><button class="btn-red" data-m="radius_stop">إيقاف</button><button class="btn-orange" data-m="radius_restart">إعادة تشغيل</button>')}
    ${sec("الحماية ضد المتطفلين", `<p>الحالة : ${st(m.fail2ban)}</p><p class="muted">ملحوظة: لا يجب أن يكون هناك NAT في الشبكة الخاصة بك، حيث يمكن أن يؤدي ذلك إلى حظر العناوين IP بشكل غير صحيح.</p>`, '<button class="b-light soon">تشغيل</button><button class="btn-red soon">إيقاف</button>')}
    ${sec("الحظر عند تخطي حالات الدخول", `<p>الحالة : ${st(true)} — 5 محاولات ثم 15 دقيقة</p>`, "")}
    ${sec("Allow root access via SSH", `<p>الحالة : ${st(m.ssh)} (Port: 22) — بالمفتاح فقط</p>`, "")}
    ${sec("إغلاق كل الجلسات", `<p>الجلسات المفتوحة على الراديوس: ${m.sessions}</p>`, '<button class="b-light" data-m="close_sessions">إغلاق الكل</button>')}
    ${sec("إخراج كل المدراء", "<p>تسجيل خروج كل مستخدمي اللوحة ما عداك</p>", '<button class="b-light" data-m="logout_managers">إخراج الكل</button>')}
    ${sec("بيانات الربط", "<p>الماكات وعناوين الـ IP المحفوظة للمشتركين المحليين</p>", '<button class="btn-red" data-m="clear_macs">حذف الماكات المحفوظة للمشتركين</button><button class="btn-red" data-m="clear_ips">حذف عناوين الايبي المحفوظة للمشتركين</button>')}
    ${sec("ضغط البيانات", "<p>تنظيف الجداول وتصغير القاعدة</p>", '<button class="b-light soon">ضغط البيانات للمشتركين</button><button class="b-light soon">ضغط البيانات للكروت</button>')}
    ${sec("فصل كل المتصلين", "<p>كل مشترك متصل على الراديوس الجديد يتفصل ويرجع يدخل لوحده (Live بس).</p>", '<button class="btn-red" id="mDisc">افصل الكل</button>')}
    ${sec("ربط سيرفر ميكروتيك", `<p>الأوامر اللي بتربط سيرفر PPPoE بالراديوس ده (RouterOS 6). <b>ما تتطبقش على سيرفر شغال غير بعد موافقة المالك.</b></p>
      <pre id="mScript" class="hidden" dir="ltr" style="white-space:pre-wrap;background:#0f172a;color:#e2e8f0;padding:10px;border-radius:8px"></pre>`, '<button class="b-light" id="mGen">طلّع الأوامر</button><button class="b-light hidden" id="mCopy">نسخ</button>')}
    ${sec("Power Control", `<p>إصدار النظام ${esc(m.version)}</p>`, '<button class="btn-orange" id="mReboot">إعادة تشغيل السيرفر</button>')}
  </div></div>`);
  const typed = (ask, word, action) => async () => {
    const t = prompt(ask + "\n\nاكتب: " + word);
    if (t === null) return;
    const r = await act(() => api("maintenance", { method: "POST", body: { Action: action, Confirm: t.trim() } }), "تم");
    if (r && r.sessions !== undefined) toast("اتفصل " + r.sessions + " مشترك");
  };
  $("#mDisc").onclick = typed("هيتفصل كل المشتركين المتصلين على الراديوس الجديد.", "نعم افصل الكل", "disconnect_all");
  $("#mReboot").onclick = typed("السيرفر هيعمل ريستارت والراديوس هيقف دقيقة تقريبًا.", "نعم اعمل ريستارت", "reboot");
  $("#mGen").onclick = async () => { const r = await act(() => api("maintenance?script=1")); $("#mScript").textContent = r.script; $("#mScript").classList.remove("hidden"); $("#mCopy").classList.remove("hidden"); };
  $("#mCopy").onclick = () => navigator.clipboard.writeText($("#mScript").textContent).then(() => toast("اتنسخ"));
  document.querySelectorAll("[data-m]").forEach(b => b.onclick = async () => { if (!confirm(b.textContent + "؟")) return; await act(() => api("maintenance", { method: "POST", body: { Action: b.dataset.m } }), "تم"); PAGES.maintenance(); });
  document.querySelectorAll(".soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
};

PAGES.genadj = async () => {
  const FIELDS = [["", "اختر شيئاً"], ["profile", "اسم العرض"], ["manager", "المدير التابع له"], ["expiration", "تاريخ الانتهاء"], ["num_doc", "رقم العقد"], ["address", "العنوان"], ["notes", "ملاحظات"], ["mobile", "رقم الهاتف"]];
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-edit"></i> تعديلات عامة</span></div><div class="adv-body"><form id="gaf"><div class="g12">
      <div class="fg s3"><label>العنصر</label><select name="Field">${FIELDS.map(([k, t]) => `<option value="${k}">${t}</option>`).join("")}</select></div>
      <div class="fg s4"><label>القيمة #</label><input name="Match" placeholder="القيمة الحالية"></div>
      <div class="fg s4"><label>قيمة جديدة</label><input name="Value"></div>
      <div class="fg s1"><label>&nbsp;</label><button class="b-light">عرض النتائج</button></div></div>
      <p class="muted">* يجب اخذ نسخة احتياطية قبل التعامل مع التعديلات العامة من خلال صفحة النسخ الإحتياطي</p>
      <div id="gaOut"></div></form><button class="btn-plain" id="gaLog">سجل التراجع عن العمليات</button></div></div>`);
  $("#gaf").onsubmit = async e => {
    e.preventDefault(); const f = formData(e.target);
    if (!f.Field) return toast("اختار العنصر", true);
    const p = await act(() => api("subscribers/genadj?preview=1", { method: "POST", body: f }));
    $("#gaOut").innerHTML = `<div class="alert" style="background:#3c8dbc">تم إيجاد ${p.count} مشترك قيمة «${esc(FIELDS.find(x => x[0] === f.Field)[1])}» عندهم = «${esc(f.Match)}». <button type="button" class="b-green" id="gaGo">تطبيق التعديل على ${p.count}</button></div>`;
    $("#gaGo").onclick = async () => { if (!confirm(`تطبيق على ${p.count} مشترك؟`)) return; const r = await act(() => api("subscribers/genadj", { method: "POST", body: f })); toast(`اتعدل ${r.n} مشترك`); $("#gaOut").innerHTML = ""; };
  };
  $("#gaLog").onclick = () => { location.hash = "audit"; };
};
