// "قائمة المتصلين" — ADV's online users page (same columns, filters and buttons).
"use strict";
const dur = s => { s = +s || 0; const d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60), x = s % 60; return (d ? d + "d" : "") + (h ? h + "h" : "") + (m ? m + "m" : "") + (!d && !h ? x + "s" : ""); };
const bts = b => bytesTxt((+b || 0) / 1048576);
PAGES.online = async () => {
  const d = await api("online");
  const shadow = ME.mode === "shadow";
  const all = shadow ? d.adv : d.own.map(o => ({ username: o.username, ip: o.framedipaddress, nas: o.nasipaddress, mac: o.callingstationid, since: o.acctstarttime,
    bytes_in: o.acctinputoctets, bytes_out: o.acctoutputoctets }));
  const nasCount = {}; all.forEach(o => { const k = o.nas_name || o.nas || "-"; nasCount[k] = (nasCount[k] || 0) + 1; });
  const st = { nas: "", prof: "", q: "", size: 25, page: 1 };
  const sel = new Set();
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-lightbulb-o"></i> قائمة المتصلين <small id="oFound"></small></span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body">
      <div class="adv-row"><div class="adv-f"><select id="fNas"><option value="">كل السيرفرات</option>${Object.entries(nasCount).sort((a, b) => b[1] - a[1]).map(([k, n]) => `<option value="${esc(k)}">${esc(k)} (${n})</option>`).join("")}</select></div>
        <div class="adv-f"><select id="fProf">${options(PROFILES, 0, "name", "كل العروض")}</select></div>
        <div class="adv-f"><select id="fConn"><option>الكل</option><option>PPPOE</option><option>هوت سبوت</option><option>لديه سرعة خاصة</option></select></div></div>
      <div class="adv-actions" style="margin:0 0 12px">
        <button class="btn-hoki" id="o360">°360 عرض</button><button class="btn-blue2" id="oMac">إغلاق بالماك</button><button class="btn-blue2" id="oIp">تثبيت الايبى ادرس</button>
        <button class="btn-hoki" id="oEdit">تعديل</button><button class="btn-hoki soon">اختبار اتصال</button><button class="btn-hoki" id="oMsg">إرسال رسالة</button>
        <button class="btn-hoki" id="oPrint">طباعة إيصال</button><button class="btn-red" id="oDisc">فصل الإتصال</button><button class="btn-teal" id="oOpen">فتح عنوان IP</button></div>
      <div class="adv-tools"><label>إظهار <select id="oSz">${[10, 25, 50, 100, 250, 500, 1000, 2000, 5000, 10000].map(n => `<option value="${n}" ${n === 25 ? "selected" : ""}>${n} في الصفحة</option>`).join("")}</select> نتائج</label>
        <label>بحث: <input id="oQ"></label></div>
      <div class="tablewrap"><table class="adv-table" id="oT"></table></div>
      <div class="adv-tools"><div id="oInfo" class="muted"></div><div class="adv-pager" id="oPg"></div></div>
    </div></div>`);
  const draw = () => {
    const pn = st.prof ? (PROFILES.find(p => +p.id === +st.prof) || {}).name : "";
    const rows = all.filter(o => (!st.nas || (o.nas_name || o.nas || "-") === st.nas) && (!pn || o.profile === pn) &&
      (!st.q || (o.username + " " + (o.firstname || "") + " " + (o.ip || "") + " " + (o.mac || "")).toLowerCase().includes(st.q)));
    const pages = Math.max(1, Math.ceil(rows.length / st.size)); st.page = Math.min(st.page, pages);
    const view = rows.slice((st.page - 1) * st.size, st.page * st.size);
    $("#oFound").textContent = `( ${fmt(rows.length)} متصل )`;
    $("#oT").innerHTML = `<thead><tr><th class="c-chk"></th><th>#</th><th>اسم المستخدم</th><th>الاسم الأول</th><th>الاسم الأخير</th><th>الباقة</th><th>العرض الحالي</th><th>عنوان IP</th><th>NAS</th>
      <th>سرعة التحميل</th><th>سرعة الرفع</th><th>وقت الجلسة</th><th>الوقت المتبقي</th><th>تحميل</th><th>رفع</th><th>عنوان MAC</th><th>المدير</th><th>انشئ بواسطة</th><th>معرف الخدمة</th><th>منفذ الاتصال للجهاز</th></tr></thead>
      <tbody>${view.map((o, i) => { const left = o.expiration ? (new Date(o.expiration.replace(" ", "T")) - new Date()) / 1000 : 0;
        return `<tr><td class="c-chk"><input type="checkbox" data-o="${o.sid || ""}" ${sel.has(+o.sid) ? "checked" : ""}></td><td class="c-n" style="background:#809fff">${(st.page - 1) * st.size + i + 1}</td>
        <td><a class="u-pill" data-open="${o.sid || ""}">${esc(o.username)}</a></td><td>${esc(o.firstname || "")}</td><td>${esc(o.lastname || "")}</td><td>${esc(o.profile || "")}</td><td>${esc(o.profile || "")}</td>
        <td><b>${esc(o.ip)}</b></td><td>${esc(o.nas_name || o.nas || "")}</td><td>${mbps(o.down_speed)}</td><td>${mbps(o.up_speed)}</td><td>${dur(o.session_sec)}</td><td>${left > 0 ? dur(Math.floor(left)) : "---"}</td>
        <td>${bts(o.bytes_out)}</td><td>${bts(o.bytes_in)}</td><td dir="ltr">${esc(o.mac || "")}</td><td>${esc(o.manager || "admin")}</td><td>${esc((MANAGERS.find(m => String(m.adv_id) === String(o.created_by)) || {}).username || o.created_by || "")}</td>
        <td>${esc(o.serv_name || "")}</td><td>${esc(o.portid || "")}</td></tr>`; }).join("") || '<tr><td colspan="20" class="empty">لا توجد بيانات</td></tr>'}</tbody>`;
    $("#oInfo").textContent = `إظهار ${rows.length ? (st.page - 1) * st.size + 1 : 0} إلى ${Math.min(rows.length, st.page * st.size)} من أصل ${fmt(rows.length)} مُدخل`;
    const p = st.page, nums = [...new Set([1, pages, p - 1, p, p + 1, p + 2].filter(n => n >= 1 && n <= pages))].sort((a, b) => a - b); let last = 0;
    $("#oPg").innerHTML = `<a class="${p <= 1 ? "dis" : ""}" data-pg="${p - 1}">السابقة</a>` + nums.map(n => { const gap = n - last > 1 ? "<span>…</span>" : ""; last = n; return gap + `<a class="${n === p ? "cur" : ""}" data-pg="${n}">${n}</a>`; }).join("") + `<a class="${p >= pages ? "dis" : ""}" data-pg="${p + 1}">التالية</a>`;
    $("#oPg").querySelectorAll("[data-pg]").forEach(a => a.onclick = () => { const n = +a.dataset.pg; if (n >= 1 && n <= pages) { st.page = n; draw(); } });
    document.querySelectorAll("[data-o]").forEach(c => c.onchange = () => c.checked ? sel.add(+c.dataset.o) : sel.delete(+c.dataset.o));
    document.querySelectorAll("[data-open]").forEach(a => a.onclick = () => { if (a.dataset.open) location.hash = "user?id=" + a.dataset.open; });
  };
  const one = () => { const id = [...sel][0]; if (!id) { toast("اختار مشترك من الجدول", true); return null; } return id; };
  $("#fNas").onchange = e => { st.nas = e.target.value; st.page = 1; draw(); };
  $("#fProf").onchange = e => { st.prof = +e.target.value; st.page = 1; draw(); };
  $("#oSz").onchange = e => { st.size = +e.target.value; st.page = 1; draw(); };
  $("#oQ").oninput = e => { st.q = e.target.value.trim().toLowerCase(); st.page = 1; draw(); };
  $("#reload").onclick = () => PAGES.online();
  $("#o360").onclick = () => { const id = one(); if (id) openSubscriber(id); };
  $("#oEdit").onclick = () => { const id = one(); if (id) location.hash = "user?id=" + id; };
  $("#oPrint").onclick = () => { const id = one(); if (id) printLastInvoice(id); };
  $("#oOpen").onclick = () => { const id = one(); if (!id) return; const o = all.find(x => +x.sid === id); if (o && o.ip) window.open("http://" + o.ip, "_blank", "noopener"); };
  const row = id => all.find(x => +x.sid === id) || {};
  const bulk1 = async (Action, Value, ok) => { const id = one(); if (!id) return; await act(() => api("subscribers/bulk", { method: "POST", body: { Action, IDs: [id], Value } }), ok); };
  $("#oMac").onclick = () => { const id = one(); if (id && confirm(`تثبيت الماك ${row(id).mac} على ${row(id).username}؟`)) bulk1("mac", row(id).mac, "اتثبت الماك"); };
  $("#oIp").onclick = () => { const id = one(); if (id && confirm(`تثبيت الـ IP ${row(id).ip} على ${row(id).username}؟`)) bulk1("ip", row(id).ip, "اتثبت الـ IP"); };
  $("#oMsg").onclick = () => { const id = one(); if (id) USER_OPS.send_msg(id); };
  $("#oDisc").onclick = () => { const id = one(); if (id) USER_OPS.disconnect_user(id); };
  document.querySelectorAll("button.soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
  draw();
};
