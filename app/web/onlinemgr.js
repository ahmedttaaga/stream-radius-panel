// "المدراء المتصلين" — ADV's online managers page: sessions of the panel itself.
"use strict";
PAGES.onlinemgr = async () => {
  const rows = await api("online-managers");
  let filter = "all";
  const sel = new Set();
  const since = s => { if (!s) return "—"; const d = (Date.now() - new Date(s.replace(" ", "T"))) / 1000; return d < 60 ? "منذ ثوان" : d < 3600 ? `منذ ${Math.floor(d / 60)} دقيقة` : d < 86400 ? `منذ ${Math.floor(d / 3600)} ساعة` : `منذ ${Math.floor(d / 86400)} يوم`; };
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-circle"></i> المدراء المتصلين</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body"><div class="adv-actions" style="margin:0 0 12px">
      <button class="b-light" id="mLogout">تسجيل خروج</button><button class="btn-red soon">إعادة توجيه</button><button class="b-green" id="mView">مشاهدة</button>
      <span class="spacer"></span><button class="btn-unified on" data-f="all">الكل</button><button class="btn-aqua" data-f="online">متصل</button><button class="b-green" data-f="active">نشط</button></div>
      <div class="tablewrap"><table class="adv-table" id="omT"></table></div></div></div>`);
  const draw = () => {
    const list = rows.filter(x => filter === "all" || (filter === "online" ? x.online : !x.online));
    $("#omT").innerHTML = `<thead><tr><th class="c-chk"></th><th>#</th><th>اسم المدير</th><th>الاسم بالكامل</th><th>آخر تصفح</th><th>آخر عملية</th><th>مدة الجلسة</th><th>أول اتصال</th><th>آخر تحديث</th><th>عنوان IP</th><th>المتصفح</th><th>OS</th></tr></thead>
      <tbody>${list.map((x, i) => `<tr><td class="c-chk"><input type="checkbox" data-t="${esc(x.token)}" ${x.me ? "disabled" : ""}></td><td class="c-n" style="background:${x.online ? "#00c0ef" : "#2dd22d"}">${i + 1}</td>
        <td>${esc(x.username)}${x.me ? ' <span class="mini blue">أنت</span>' : ""}</td><td>${esc(x.full_name)}</td><td>${esc(x.last_action || "—")}</td><td>${since(x.last_seen)}</td>
        <td>${dur(Math.max(0, (new Date((x.last_seen || x.created).replace(" ", "T")) - new Date(x.created.replace(" ", "T"))) / 1000))}</td><td>${advDate(x.created)}</td><td>${advDate(x.last_seen)}</td>
        <td dir="ltr">${esc(x.ip)}</td><td>${esc(x.browser)}</td><td>${esc(x.os)}</td></tr>`).join("") || '<tr><td colspan="12" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}</tbody>`;
    document.querySelectorAll("[data-t]").forEach(c => c.onchange = () => c.checked ? sel.add(c.dataset.t) : sel.delete(c.dataset.t));
  };
  document.querySelectorAll("[data-f]").forEach(b => b.onclick = () => { filter = b.dataset.f; document.querySelectorAll("[data-f]").forEach(x => x.classList.toggle("on", x === b)); draw(); });
  $("#reload").onclick = () => PAGES.onlinemgr();
  $("#mLogout").onclick = async () => { if (!sel.size) return toast("اختار جلسة", true); for (const t of sel) await api("online-managers/logout", { method: "POST", body: { Token: t } }); toast("تم تسجيل الخروج"); PAGES.onlinemgr(); };
  $("#mView").onclick = () => { if (!sel.size) return toast("اختار جلسة", true); const x = rows.find(r => sel.has(r.token)); modal(`<h3>${esc(x.username)}</h3><div class="kv"><div>IP</div><div dir="ltr">${esc(x.ip)}</div><div>المتصفح</div><div dir="ltr">${esc(x.ua)}</div><div>آخر صفحة</div><div>${esc(x.last_action)}</div></div>`); };
  document.querySelectorAll("button.soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
  draw();
};
