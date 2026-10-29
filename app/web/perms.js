// "صلاحيات الموزعين" (المجموعات الامنية) and "توجيه البورتات" — same layout as ADV.
"use strict";
PAGES.perms = async () => {
  const groups = await api("security-groups");
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-key"></i> المجموعات الامنية <small>(تم إيجاد ${groups.length} نتائج )</small></span></div><div class="adv-body">
    <div class="adv-actions" style="margin:0 0 12px"><button class="btn-hoki soon"><i class="fa fa-plus"></i> إضافة</button><button class="btn-blue2 soon">تعديل</button>
      <button class="btn-blue2" id="toMgr">قائمة المدراء</button><button class="btn-red soon">حذف</button></div>
    <div class="adv-tools"><div class="adv-tbtns"><button>CSV</button><button>EXCEL</button><button onclick="print()">طباعة</button><button>عرض/إخفاء</button></div><label>بحث <input id="gq"></label></div>
    <div class="tablewrap"><table class="adv-table" id="gT"></table></div>
    <div class="adv-tools"><div class="muted">إظهار 1 إلى ${groups.length} من أصل ${groups.length} مُدخل</div><div class="adv-pager"><a class="dis">السابقة</a><a class="cur">1</a><a class="dis">التالية</a></div></div></div></div>`);
  const draw = q => { $("#gT").innerHTML = `<thead><tr><th class="c-chk"><input type="checkbox"></th><th>#</th><th>اسم المجموعة</th><th>عدد المدراء</th><th>إجمالي الصلاحيات</th><th>انشى في</th></tr></thead>
    <tbody>${groups.filter(g => !q || String(g.name).includes(q)).map((g, i) => `<tr><td class="c-chk"><input type="radio" name="g"></td><td>${i + 1}</td><td>${esc(g.name)}</td><td>${g.managers}</td><td>${g.perms}</td><td>${esc(g.created || "")}</td></tr>`).join("")
    || '<tr><td colspan="6" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}</tbody>`; };
  $("#gq").oninput = e => draw(e.target.value.trim());
  $("#toMgr").onclick = () => { location.hash = "managers"; };
  document.querySelectorAll("button.soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
  draw("");
};

PAGES.ports = async () => {
  const d = await api("online"), nasRows = await api("nas");
  const nasName = ip => ((nasRows.adv || []).find(n => n.ip === ip) || {}).name || ip;
  // static routes like ADV's table: each customer /20 pool → the NAS that serves it (derived from the live sessions)
  const routes = {};
  (d.adv || []).forEach(o => { const m = /^(\d+)\.(\d+)\.(\d+)\./.exec(o.ip || ""); if (!m) return; const net = `${m[1]}.${m[2]}.${(+m[3] >> 4) << 4}.0/20`; routes[net] = routes[net] || {}; routes[net][o.nas] = (routes[net][o.nas] || 0) + 1; });
  const list = Object.entries(routes).map(([net, by]) => ({ net, gw: Object.entries(by).sort((a, b) => b[1] - a[1])[0][0] })).sort((a, b) => b.net.localeCompare(a.net));
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-random"></i> توجيه البورتات</span></div><div class="adv-body">
    <h3 class="form-section">طريقة الاتصال الخارجي</h3>
    <div class="pf-cards"><div class="pf-card on"><b>عنوان Public IP مباشر</b><small>استخدام عنوان السيرفر العام مباشرةً.</small></div><div class="pf-card"><b>نسخة محلية عبر ADV Tunnel</b><small>استخدام سيرفر التنل للوصول إلى نسخة داخل شبكة محلية.</small></div></div>
    <div class="kv" style="margin:12px 0"><div>الـ IP العام:</div><div>${esc(location.hostname)}</div><div>خدمة SSTP (المنفذ 4444):</div><div><span class="mini gray">غير مثبتة</span></div><div>نطاق البورتات المتاح:</div><div>40000 - 49999</div></div>
    <div class="adv-actions" style="margin-bottom:14px"><button class="b-green soon">تنزيلُ تطبيقِ Windows</button><button class="b-green soon">تنزيلُ تطبيقِ Android</button><button class="b-green soon">تثبيت الإصلاح التلقائي</button><button class="btn-plain soon">إلغاء التثبيت</button></div>
    <h3 class="form-section">الطرق الثابتة (Static Routes)</h3>
    <div class="adv-row"><div class="adv-f"><label>الشبكة</label><input id="srNet" dir="ltr" placeholder="100.64.0.0/20"></div><div class="adv-f"><label>البوابة</label><input id="srGw" dir="ltr"></div>
      <div class="adv-f"><label>الواجهة</label><select><option>— تلقائي —</option><option>eth0</option><option>eth1</option></select></div><div class="adv-f"><label>الوصف</label><input></div>
      <div class="adv-actions"><button class="btn-lblue soon">إضافةٌ</button><button class="btn-red soon">حذف</button></div></div>
    <div class="tablewrap"><table class="adv-table"><thead><tr><th>#</th><th>الشبكة</th><th>البوابة</th><th>الواجهة</th><th>الوصف</th><th>الحالة</th><th>إجراءات</th></tr></thead>
      <tbody>${list.map((r, i) => `<tr><td>${list.length - i}</td><td dir="ltr">${esc(r.net)}</td><td dir="ltr">${esc(r.gw)}</td><td>—</td><td>${esc(nasName(r.gw))}</td><td><span class="mini green">مُطبَّق</span></td><td><a class="soon">حذف</a></td></tr>`).join("") || '<tr><td colspan="7" class="empty">لا توجد بيانات</td></tr>'}</tbody></table></div>
    <h3 class="form-section">توجيه البورتات (Port Forwarding)</h3>
    <div class="adv-actions" style="margin-bottom:10px"><button class="btn-lblue soon">من قائمة الأجهزة</button><button class="btn-pale2 soon">IP يدوي (بدون إضافة جهاز)</button></div>
    <div class="adv-row"><div class="adv-f"><label>NAS</label><select><option>تحديد...</option>${(nasRows.adv || []).map(n => `<option>${esc(n.name)} — ${esc(n.ip)}</option>`).join("")}</select></div>
      <div class="adv-f"><label>البروتوكول</label><select><option>TCP</option><option>UDP</option></select></div><div class="adv-f"><label>البورت الداخلي</label><input dir="ltr"></div><div class="adv-f"><label>البورت الخارجي</label><input dir="ltr" placeholder="40000-49999"></div>
      <div class="adv-actions"><button class="btn-lblue soon">إضافة</button><button class="btn-orange soon">إصلاح الكل</button><button class="btn-plain soon">تحديث</button></div></div>
    <div class="tablewrap"><table class="adv-table"><thead><tr><th>#</th><th>NAS</th><th>البروتوكول</th><th>داخلي</th><th>خارجي</th><th>الحالة</th><th>إجراءات</th></tr></thead><tbody><tr><td colspan="7" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr></tbody></table></div>
    <p class="muted">التوجيه الفعلي محتاج خدمة SSTP على السيرفر ده؛ هتتركب في مرحلة النقل.</p></div></div>`);
  document.querySelectorAll(".soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
};
