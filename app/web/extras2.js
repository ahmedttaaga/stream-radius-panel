// More ADV pages: مستخدمو التطبيق المتصلون، الكروت المتصلة، IP Pools، اسعار العروض (per-manager price matrix).
"use strict";
PAGES.appusers = async () => {
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-mobile"></i> مستخدمو التطبيق المتصلون <small>(تم إيجاد 0 مشتركين)</small></span></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="btn-hoki soon">إشعار التطبيق</button></div>
    <div class="adv-tools"><div class="adv-tbtns"><button>CSV</button><button>Excel</button><button onclick="print()">طباعة</button></div><label>إظهار <select>${[10, 25, 50, 100, 250, 1000, 2000].map(n => `<option>${n}</option>`).join("")}</select> نتائج</label></div>
    <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>اسم المستخدم</th><th>الباقة</th><th>الحالة</th><th>شوهد آخر مرة</th><th>الشاشة الأخيرة</th></tr></thead>
      <tbody><tr><td colspan="6" class="empty">تطبيق المشتركين هيتربط بالنظام الجديد في مرحلة النقل</td></tr></tbody></table></div></div></div>`);
  document.querySelectorAll(".soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
};

PAGES.onlinecards = async () => {
  const d = await api("online"), series = await api("cardseries");
  const names = new Set(series.map(r => { try { return JSON.parse(r.raw).name_ser; } catch (_) { return ""; } }));
  const rows = (d.adv || []).filter(o => !o.sid || /^(cafe|card)/i.test(o.profile || "") || names.has(o.username));
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-credit-card"></i> الكروت المتصلة</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="btn-blue2 soon">إغلاق بالماك</button><button class="btn-hoki soon">اختبار اتصال</button><button class="btn-red soon">فصل الإتصال</button></div>
    <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>حزمة</th><th>طريقة الإحتساب</th><th>اسم المستخدم</th><th>الباقة</th><th>عنوان IP</th><th>NAS</th><th>سرعة التحميل</th><th>سرعة الرفع</th>
      <th>وقت الجلسة</th><th>الوقت المتبقي</th><th>تحميل</th><th>رفع</th><th>عنوان MAC</th><th>الشركة</th><th>المدير</th><th>انشئ بواسطة</th></tr></thead>
      <tbody>${rows.map((o, i) => `<tr><td class="c-chk"><input type="checkbox"></td><td class="c-n" style="background:#809fff">${i + 1}</td><td>—</td><td>من اول إتصال</td><td>${esc(o.username)}</td><td>${esc(o.profile || "")}</td>
        <td><b>${esc(o.ip)}</b></td><td>${esc(o.nas_name || o.nas)}</td><td>${mbps(o.down_speed)}</td><td>${mbps(o.up_speed)}</td><td>${dur(o.session_sec)}</td><td>---</td><td>${bts(o.bytes_out)}</td><td>${bts(o.bytes_in)}</td>
        <td dir="ltr">${esc(o.mac)}</td><td></td><td>${esc(o.manager || "admin")}</td><td></td></tr>`).join("") || '<tr><td colspan="18" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}</tbody></table></div></div></div>`);
  $("#reload").onclick = () => PAGES.onlinecards();
  document.querySelectorAll(".soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
};

PAGES.ippool = async () => {
  const d = await api("online"), nasRows = await api("nas");
  const nasName = ip => ((nasRows.adv || []).find(n => n.ip === ip) || {}).name || ip;
  const pools = {};
  (d.adv || []).forEach(o => { const m = /^(\d+)\.(\d+)\.(\d+)\./.exec(o.ip || ""); if (!m) return; const k = `${m[1]}.${m[2]}.${(+m[3] >> 4) << 4}.0/20`; pools[k] = pools[k] || { net: k, used: 0, nas: {} }; pools[k].used++; pools[k].nas[o.nas] = (pools[k].nas[o.nas] || 0) + 1; });
  const list = Object.values(pools).sort((a, b) => a.net.localeCompare(b.net, undefined, { numeric: true }));
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-sitemap"></i> IP Pools <small>(تم إيجاد ${list.length} نتائج )</small></span></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="btn-hoki soon"><i class="fa fa-plus"></i> إضافة</button><button class="btn-blue2 soon">تعديل</button><button class="btn-red soon">حذف</button></div>
    <div class="adv-tools"><label>إظهار <select>${[10, 25, 50, 100, 200, 500].map(n => `<option>${n} في الصفحة</option>`).join("")}</select> نتائج</label><label>بحث <input></label></div>
    <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>الاسم</th><th>حزمة العناوين</th><th>عدد العناوين</th><th>عدد العناوين المستخدمة</th><th>تاريخ التحديث</th><th>تاريخ الإنشاء</th><th>تجمع العناوين التالي</th></tr></thead>
      <tbody>${list.map((p, i) => { const top = Object.entries(p.nas).sort((a, b) => b[1] - a[1])[0][0]; return `<tr><td class="c-chk"><input type="radio" name="pool"></td><td>${i + 1}</td><td>${esc(nasName(top))}</td><td dir="ltr">${esc(p.net)}</td><td>4,094</td><td>${p.used}</td><td>—</td><td>—</td><td>—</td></tr>`; }).join("")
      || '<tr><td colspan="9" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}</tbody></table></div>
    <p class="muted">الـ Pools متعرّفة على سيرفرات الميكروتيك نفسها؛ الجدول ده بيتحسب من المشتركين المتصلين (كل /20 والسيرفر اللي بيخدمه).</p></div></div>`);
  document.querySelectorAll(".soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
};

PAGES.pricing = async () => {
  const profs = await api("profiles/full"), mgrs = (await api("managers")).filter(m => m.username !== "admin");
  const S = await api("settings").then(d => Object.fromEntries(d.settings.map(x => [x.k, x.v]))).catch(() => ({}));
  let custom = {}; try { custom = JSON.parse(S.pricing_custom || "{}"); } catch (_) {}
  const price = (m, p) => (custom[m.id] || {})[p.id] ?? p.price;
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-tags"></i> اسعار العروض</span></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="btn-blue2" id="prEdit">تعديل</button><button class="btn-hoki" id="prRestore">استعادة</button><button class="btn-red2" id="prRestoreAll">استعادة الكل</button>
      <span class="spacer"></span><label>المدير: <select id="prMgr"><option value="0">admin (Administrator)</option>${mgrs.map(m => `<option value="${m.id}">${esc(m.username)} (${esc(m.full_name)})</option>`).join("")}</select></label></div>
    <div class="adv-tools"><div class="adv-tbtns"><button>CSV</button><button>EXCEL</button><button onclick="print()">طباعة</button><button>عرض/إخفاء</button></div><label>بحث <input id="prq"></label></div>
    <div class="tablewrap"><table class="adv-table" id="prT"></table></div></div></div>`);
  const draw = () => {
    const q = $("#prq").value.trim().toLowerCase();
    $("#prT").innerHTML = `<thead><tr><th class="c-chk"></th><th>#</th><th>اسم المدير</th>${profs.map(p => `<th>${esc(p.name)}</th>`).join("")}</tr></thead><tbody>${mgrs.filter(m => !q || m.username.toLowerCase().includes(q)).map((m, i) =>
      `<tr><td class="c-chk"><input type="radio" name="prm" value="${m.id}"></td><td>${i + 1}</td><td>${esc(m.username)}</td>${profs.map(p => `<td class="${(custom[m.id] || {})[p.id] !== undefined ? "c-custom" : ""}">${Number(price(m, p)).toFixed(2)}</td>`).join("")}</tr>`).join("")}</tbody>`;
  };
  const picked = () => { const r = document.querySelector("[name=prm]:checked"); if (!r) { toast("اختار مدير", true); return null; } return mgrs.find(m => m.id === +r.value); };
  const save = async () => { await act(() => api("settings", { method: "PUT", body: { pricing_custom: JSON.stringify(custom) } }), "تم الحفظ"); draw(); };
  $("#prEdit").onclick = () => {
    const m = picked(); if (!m) return;
    const md = modal(`<h3>اسعار العروض — ${esc(m.username)}</h3><form id="prf" class="tablewrap"><table class="adv-table"><thead><tr><th>العرض</th><th>السعر الأساسي</th><th>سعر المدير</th></tr></thead>
      <tbody>${profs.map(p => `<tr><td>${esc(p.name)}</td><td>${Number(p.price).toFixed(2)}</td><td><input name="p${p.id}" type="number" step="0.01" value="${Number(price(m, p)).toFixed(2)}"></td></tr>`).join("")}</tbody></table>
      <div class="form-actions"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
    $("#prf", md).onsubmit = e => { e.preventDefault(); const f = formData(e.target); custom[m.id] = {}; profs.forEach(p => { const v = +f["p" + p.id]; if (v !== +p.price) custom[m.id][p.id] = v; }); closeModal(); save(); };
  };
  $("#prRestore").onclick = () => { const m = picked(); if (m && confirm(`استعادة الأسعار الأساسية لـ ${m.username}؟`)) { delete custom[m.id]; save(); } };
  $("#prRestoreAll").onclick = () => { if (confirm("استعادة الأسعار الأساسية لكل المدراء؟")) { custom = {}; save(); } };
  $("#prq").oninput = draw;
  draw();
};
