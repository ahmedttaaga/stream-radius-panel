// "الشكاوى" — ADV's tickets list. In shadow mode it shows ADV's tickets (read-only); our own tickets otherwise.
"use strict";
PAGES.tickets = async () => {
  const shadow = ME.mode === "shadow";
  let rows = [];
  if (shadow && ME.admin.role === "admin") {
    try { rows = repRows(await api("adv/get_tickets?limit=1000")).map(t => ({ id: t.id, username: t.username || t.user_name || t.id_user, subject: t.title || t.subject,
      status: t.status, num: t.id, last: t.last_reply_by || t.last_by || t.username || "", replies: t.replies_count || t.count_replies || t.replies || 0 })); } catch (e) { toast(e.message, true); }
  } else {
    rows = (await api("tickets")).map(t => ({ id: t.id, username: t.username, subject: t.subject, status: t.status, num: t.id, last: t.created_by, replies: 0, local: 1 }));
  }
  const stAr = s => ({ open: "مفتوح", closed: "مغلق", pending: "معلق", sloved: "تم الحل", solved: "تم الحل", "0": "مفتوح", "1": "مغلق" })[String(s)] || s;
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-exclamation-circle"></i> الشكاوى</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body"><div class="adv-actions" style="margin:0 0 12px">
      <button class="btn-pale" id="tView">عرض</button><button class="btn-pale soon">حذف</button><button class="btn-pale" id="tStatus">تغيير الحالة</button><button class="btn-emerald" id="tNew">فتح للعميل</button></div>
      <div class="tablewrap"><table class="adv-table"><thead><tr><th class="c-chk"></th><th>#</th><th>اسم المستخدم</th><th>العنوان</th><th>الحالة</th><th>رقم</th><th>آخر رد بواسطة</th><th>عدد الردود</th></tr></thead>
      <tbody>${rows.map((t, i) => `<tr><td class="c-chk"><input type="radio" name="tk" value="${esc(t.id)}" data-local="${t.local ? 1 : ""}"></td><td>${i + 1}</td><td>${esc(t.username)}</td>
        <td>${esc(t.subject)}</td><td><span class="mini ${/open|^0$/.test(t.status) ? "green" : "gray"}">${esc(stAr(t.status))}</span></td><td>${esc(t.num)}</td><td>${esc(t.last)}</td><td>${esc(t.replies)}</td></tr>`).join("")
        || '<tr><td colspan="8" class="empty">لا توجد اي بيانات في هذا الجدول</td></tr>'}</tbody></table></div>
      <div class="adv-tools"><div class="muted">إظهار 1 إلى ${rows.length} من أصل ${rows.length} مُدخل</div></div></div></div>`);
  $("#reload").onclick = () => PAGES.tickets();
  $("#tView").onclick = async () => {
    const r = document.querySelector("[name=tk]:checked"); if (!r) return toast("اختار شكوى", true);
    if (!r.dataset.local) return toast("تفاصيل شكاوى ADV بتتفتح من ADV نفسه لحد النقل", true);
    const d = await api("tickets/" + r.value), t = d.ticket[0];
    modal(`<h3>${esc(t.subject)}</h3>${d.replies.map(x => `<div class="panel"><div class="pbody"><b>${esc(x.created_by)}</b> <span class="muted">${esc(x.created)}</span><br>${esc(x.body)}</div></div>`).join("")}`);
  };
  $("#tNew").onclick = () => {
    const m = modal(`<h3>شكوى جديدة</h3><form class="form" id="tf"><label>يوزر المشترك<input name="Username"></label><label>العنوان<input name="Subject" required></label>
      <label class="full">التفاصيل<textarea name="Body" rows="4"></textarea></label><div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
    $("#tf", m).onsubmit = async e => { e.preventDefault(); await act(() => api("tickets", { method: "POST", body: formData(e.target) }), "اتسجلت"); closeModal(); PAGES.tickets(); };
  };
  $("#tStatus").onclick = () => {
    const r = document.querySelector("[name=tk]:checked"); if (!r) return toast("اختار شكوى", true);
    if (!r.dataset.local) return toast("شكاوى ADV بتتعدل من ADV نفسه لحد النقل (وضع المراقبة)", true);
    const m = modal(`<h3>تغيير حالة الشكوى</h3><form class="form" id="tsf"><label class="full">الحالة<select name="Status"><option value="open">مفتوح</option><option value="pending">معلق</option><option value="closed">مغلق</option></select></label>
      <label class="full">ملاحظة (اختياري)<textarea name="Body" rows="2"></textarea></label><div class="form-actions full"><button type="button" class="b-cancel" data-close>إلغاء</button><button class="b-save">حفظ</button></div></form>`);
    $("#tsf", m).onsubmit = async e => { e.preventDefault(); await act(() => api(`tickets/${r.value}/reply`, { method: "POST", body: formData(e.target) }), "اتغيرت الحالة"); closeModal(); PAGES.tickets(); };
  };
  document.querySelectorAll("button.soon").forEach(b => b.onclick = () => toast("الخاصية دي هتتعمل في المرحلة الجاية", true));
};
