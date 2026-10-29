// "قائمة المدراء" — ADV's managers list: same columns, buttons and menus.
"use strict";
PAGES.managers = async () => {
  MANAGERS = await api("managers");
  const list = MANAGERS.filter(m => m.username !== "admin").sort((a, b) => a.username.localeCompare(b.username)); // ADV lists sub-managers A→Z, not the root admin
  const sel = new Set();
  let size = 25, page = 1, q = "";
  const dd = (id, label, items, cls = "btn-chambray") => `<span class="dd"><button class="${cls}" data-dd="${id}">${label} <i class="fa fa-caret-down"></i></button>
    <div class="dd-menu" id="${id}">${items.map(([k, t]) => `<a data-mact="${k}" class="${k === "soon" ? "soon" : ""}">${t}</a>`).join("")}</div></span>`;
  C(`<div class="adv-card">
    <div class="adv-head"><span><i class="fa fa-university"></i> قائمة المدراء</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body">
      <div class="adv-actions" style="margin:0 0 12px">
        <button class="btn-hoki" data-mact="add"><i class="fa fa-plus"></i> إضافة</button>
        <button class="btn-blue2" data-mact="edit"><i class="fa fa-edit"></i> تعديل</button>
        <button class="btn-hoki" data-mact="soon">إعادة تسمية</button>
        <button class="btn-hoki" data-mact="soon">تغيير الجلسة</button>
        <button class="btn-red" data-mact="soon"><i class="fa fa-trash"></i> حذف</button>
        <button class="btn-hoki" data-mact="soon">إرسال رسالة</button>
        <button class="btn-hoki" data-mact="ledger">°360 عرض</button>
        <button class="btn-blue2" data-mact="copy">نسخ</button>
        <button class="btn-green2" data-mact="login"><i class="fa fa-key"></i> حساب دخول</button>
        <button class="btn-chambray" data-mact="soon">النسب المضافة</button>
        ${dd("mMore", "المزيد", [["soon", "تصفير عداد الانتظار (واتساب)"], ["soon", "رفع الحظر عن جميع حالات تسجيل الدخول"], ["soon", "إلغاء حظر جميع الايبيهات المحظورة"]])}
        ${dd("mTrial", "رصيد تجريبي", [["soon", "إضافة مبلغ نقدي"], ["soon", "سحب مبلغ نقدي"]])}
        ${dd("mBal", "الرصيد", [["dep", "إضافة مبلغ نقدي"], ["wd", "سحب مبلغ نقدي"], ["soon", "دفع جزئي"]])}
      </div>
      <div class="adv-tools"><label>إظهار <select id="msz">${[10, 25, 50, 100, 250, 500, 1000, 5000].map(n => `<option value="${n}" ${n === size ? "selected" : ""}>${n} في الصفحة</option>`).join("")}</select> نتائج</label>
        <label>بحث: <input id="mq"></label></div>
      <div class="tablewrap"><table class="adv-table" id="mT"></table></div>
      <div class="adv-tools"><div id="mInfo" class="muted"></div><div class="adv-pager" id="mPg"></div></div>
    </div></div>`);
  const draw = () => {
    const rows = list.filter(m => !q || (m.username + " " + m.full_name + " " + m.phone).toLowerCase().includes(q));
    const pages = Math.max(1, Math.ceil(rows.length / size)); page = Math.min(page, pages);
    const view = rows.slice((page - 1) * size, page * size);
    $("#mT").innerHTML = `<thead><tr><th class="c-chk"><input type="checkbox" id="mAll"></th><th>#</th><th>اسم المدير</th><th>الاسم بالكامل</th><th>الهاتف</th>
      <th>الرصيد (EGP)</th><th>الديون (EGP)</th><th>المدير</th><th>مجموعة</th><th>إجمالي المشتركين</th><th>وحدات تجريبية</th></tr></thead><tbody>${view.map((m, i) => `<tr>
      <td class="c-chk"><input type="checkbox" data-m="${m.id}" ${sel.has(m.id) ? "checked" : ""}></td><td>${(page - 1) * size + i + 1}</td>
      <td><a class="plink" data-medit="${m.id}">${esc(m.username)}</a></td><td>${esc(m.full_name)}</td><td>${esc(m.phone || "---")}</td>
      <td>${+m.unlimited ? "غير محدود" : fmt(m.balance)}</td><td>${Number(m.debt || 0).toFixed(2)}</td><td>${esc(m.parent || "admin")}</td><td>${esc(m.grp || "")}</td>
      <td>${m.users}</td><td>${Number(m.trial || 0).toFixed(2)}</td></tr>`).join("") || '<tr><td colspan="11" class="empty">لا توجد بيانات</td></tr>'}</tbody>`;
    $("#mInfo").textContent = `إظهار ${rows.length ? (page - 1) * size + 1 : 0} إلى ${Math.min(rows.length, page * size)} من أصل ${rows.length} مُدخل`;
    $("#mPg").innerHTML = `<a class="${page <= 1 ? "dis" : ""}" data-pg="${page - 1}">السابقة</a>` + Array.from({ length: pages }, (_, i) => `<a class="${i + 1 === page ? "cur" : ""}" data-pg="${i + 1}">${i + 1}</a>`).join("") + `<a class="${page >= pages ? "dis" : ""}" data-pg="${page + 1}">التالية</a>`;
    $("#mPg").querySelectorAll("[data-pg]").forEach(a => a.onclick = () => { const n = +a.dataset.pg; if (n >= 1 && n <= pages) { page = n; draw(); } });
    document.querySelectorAll("[data-m]").forEach(c => c.onchange = () => c.checked ? sel.add(+c.dataset.m) : sel.delete(+c.dataset.m));
    $("#mAll").onchange = e => { view.forEach(m => e.target.checked ? sel.add(m.id) : sel.delete(m.id)); draw(); };
    document.querySelectorAll("[data-medit]").forEach(a => a.onclick = () => editManager(list.find(m => m.id === +a.dataset.medit)));
  };
  const one = () => { if (sel.size !== 1) { toast("من فضلك اختر مدير واحد", true); return null; } return list.find(m => m.id === [...sel][0]); };
  const editManager = x => {
    x = x || { enabled: 1 };
    const m = modal(`<h3>${x.id ? "تعديل المدير " + esc(x.username) : "إضافة مدير"}</h3><form class="form" id="mf"><label>اسم المدير<input name="Username" value="${esc(x.username)}" required></label>
      <label>الاسم بالكامل<input name="FullName" value="${esc(x.full_name)}"></label><label>الهاتف<input name="Phone" value="${esc(x.phone)}"></label>
      <label>المدير الأعلى<select name="ParentID">${options(list.filter(z => z.id !== x.id), x.parent_id, "username", "admin")}</select></label>
      <label class="full"><span><input type="checkbox" name="Unlimited" ${+x.unlimited ? "checked" : ""} style="width:auto"> رصيد غير محدود</span></label>
      <label class="full"><span><input type="checkbox" name="Enabled" ${+x.enabled ? "checked" : ""} style="width:auto"> مفعّل</span></label>
      <div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
    $("#mf", m).onsubmit = async e => {
      e.preventDefault(); const f = formData(e.target);
      await act(() => api(x.id ? "managers/" + x.id : "managers", { method: x.id ? "PUT" : "POST", body: { ...f, ParentID: +f.ParentID, Unlimited: !!f.Unlimited, Enabled: !!f.Enabled } }), "تم الحفظ");
      closeModal(); PAGES.managers();
    };
  };
  const money = async (sign) => {
    const x = one(); if (!x) return;
    const amt = prompt(`${sign > 0 ? "إضافة" : "سحب"} مبلغ نقدي — ${x.username} (الرصيد الحالي ${fmt(x.balance)}):`); if (!amt) return;
    const reason = prompt("ملاحظة:", sign > 0 ? "شحن رصيد" : "سحب رصيد") || "";
    await act(() => api(`managers/${x.id}/balance`, { method: "POST", body: { Amount: sign * Math.abs(+amt), Reason: reason } }), "تم");
    PAGES.managers();
  };
  document.querySelectorAll("[data-mact]").forEach(b => b.onclick = async () => {
    document.querySelectorAll(".dd-menu.open").forEach(x => x.classList.remove("open"));
    switch (b.dataset.mact) {
      case "soon": return toast("الخاصية دي هتتعمل في المرحلة الجاية", true);
      case "add": return editManager();
      case "edit": { const x = one(); if (x) editManager(x); return; }
      case "copy": { const x = one(); if (x) editManager({ ...x, id: 0, username: x.username + "-copy", balance: 0 }); return; }
      case "login": { const x = one(); if (!x) return;
        const m = modal(`<h3>حساب دخول للمدير ${esc(x.username)}</h3><p class="muted">المدير هيدخل اللوحة باسم <b dir="ltr">${esc(x.username)}</b> ويشوف مشتركينه بس، ويفعّل بأسعاره الخاصة.</p>
          <form class="form" id="lgf"><label class="full">كلمة المرور (8 حروف على الأقل)<input name="Password" type="password" autocomplete="new-password" required minlength="8"></label>
          <div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
        $("#lgf", m).onsubmit = async e => { e.preventDefault(); await act(() => api(`managers/${x.id}/login`, { method: "POST", body: formData(e.target) }), "اتعمل حساب الدخول"); closeModal(); };
        return; }
      case "dep": return money(1);
      case "wd": return money(-1);
      case "ledger": { const x = one(); if (!x) return; const rows = await api(`managers/${x.id}/balance`);
        return modal(`<h3>°360 — ${esc(x.username)}</h3><div class="kv"><div>الاسم</div><div>${esc(x.full_name)}</div><div>الرصيد</div><div>${fmt(x.balance)}</div><div>المشتركين</div><div>${x.users}</div></div>
          <h3>حركة الرصيد</h3><div class="tablewrap">${table(rows, [["created", "الوقت"], ["delta", "المبلغ"], ["balance_after", "الرصيد بعدها"], ["reason", "ملاحظة"], ["created_by", "بواسطة"]])}</div>`); }
    }
  });
  document.querySelectorAll("[data-dd]").forEach(b => b.onclick = e => { e.stopPropagation(); const m = $("#" + b.dataset.dd); const o = m.classList.contains("open"); document.querySelectorAll(".dd-menu.open").forEach(x => x.classList.remove("open")); if (!o) m.classList.add("open"); });
  $("#msz").onchange = e => { size = +e.target.value; page = 1; draw(); };
  $("#mq").oninput = e => { q = e.target.value.trim().toLowerCase(); page = 1; draw(); };
  $("#reload").onclick = () => PAGES.managers();
  draw();
};
