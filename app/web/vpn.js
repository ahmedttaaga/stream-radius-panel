// "VPN الكافيهات": SSTP accounts for the cafés' MikroTik routers + the ready RouterOS script for each one.
"use strict";
PAGES.vpn = async () => {
  const [d, mgrs] = await Promise.all([api("vpn"), api("managers").catch(() => [])]);
  const mOpts = sel => `<option value="0">— بدون —</option>` + mgrs.map(m => `<option value="${m.id}" ${+sel === +m.id ? "selected" : ""}>${esc(m.username)}</option>`).join("");
  C(`<div class="dz">
    <div class="dz-card dz-head"><div class="dz-title"><span class="dz-bub big"><i class="bi bi-shield-lock-fill"></i></span>
      <div><h2>VPN الكافيهات (SSTP)</h2><small>كل كافيه راوتره بيتصل بالراديوس مباشرة من أي خط نت. النفق للراديوس بس — نت الكافيه مش بيعدي من هنا.</small></div>
      <span class="dz-status ${d.running ? "ok" : "bad"}"><span class="dz-dot"></span>${d.installed ? (d.running ? "السيرفر شغال" : "السيرفر واقف") : "مش متسطّب"}</span></div></div>
    <form class="dz-card" id="vpnSet"><div class="dz-card-h"><span><i class="bi bi-globe2"></i> العنوان اللي الكافيهات بتتصل بيه</span><button class="green">حفظ</button></div>
      <div class="g12"><div class="fg s8"><label>دومين أو IP عام</label><input name="Host" dir="ltr" value="${esc(d.host || "")}" placeholder="vpn.example.com أو 41.x.x.x"></div>
        <div class="fg s4"><label>البورت (TCP)</label><input name="Port" dir="ltr" value="${esc(d.port)}"></div></div>
      <p class="muted">لازم البورت ده يتوجّه من الراوتر الرئيسي (اللي عليه الـ IP العام) لـ <b dir="ltr">IP-السيرفر:${esc(d.port)}</b>. شبكة الأنفاق <span dir="ltr">${esc(d.net)}</span> والراديوس جوه النفق على <b dir="ltr">10.199.0.1</b>.</p></form>
    <form class="dz-card" id="vpnAdd"><div class="dz-card-h"><span><i class="bi bi-plus-circle"></i> كافيه جديد</span><button class="green"><i class="bi bi-plus"></i> إضافة</button></div>
      <div class="g12"><div class="fg s4"><label>اسم الكافيه</label><input name="Name" required></div><div class="fg s3"><label>موبايل صاحب الكافيه</label><input name="Phone" dir="ltr"></div>
        <div class="fg s3"><label>المدير / الموزع</label><select name="ManagerID">${mOpts(0)}</select></div><div class="fg s2"><label>ملاحظات</label><input name="Notes"></div></div></form>
    <div class="dz-card"><div class="dz-card-h"><span><i class="bi bi-cup-hot-fill"></i> الكافيهات (${(d.clients || []).length})</span></div>
      ${(d.clients || []).length ? `<div class="tablewrap"><table><tr><th>الكافيه</th><th>الحالة</th><th>عنوان النفق</th><th>اليوزر</th><th>الموبايل</th><th>المدير</th><th>كروت متصلة</th><th></th></tr>
        ${d.clients.map(c => `<tr><td><b>${esc(c.name)}</b>${c.notes ? `<br><small class="muted">${esc(c.notes)}</small>` : ""}</td>
          <td>${+c.enabled ? (c.connected ? `<span class="dz-pill p-green"><span class="dz-dot"></span> متصل ${esc(c.uptime || "")}</span>` : '<span class="dz-pill p-rose">مش متصل</span>') : '<span class="dz-pill p-slate">موقوف</span>'}</td>
          <td dir="ltr">${esc(c.ip)}</td><td dir="ltr">${esc(c.username)}</td><td dir="ltr">${esc(c.phone || "")}</td><td>${esc(c.manager || "")}</td><td>${fmt(c.online_cards)}</td>
          <td class="vpn-act"><button class="b-light" data-scr="${c.id}"><i class="bi bi-terminal"></i> أوامر الراوتر</button>
            <button class="b-light" data-em="${c.id}" title="تجهيز بس — ما بيتطبقش على الراوتر"><i class="bi bi-life-preserver"></i> الطوارئ + الهوتسبوت</button>
            <button class="b-light" data-act="toggle" data-id="${c.id}">${+c.enabled ? "إيقاف" : "تشغيل"}</button>
            <button class="b-light" data-act="new_password" data-id="${c.id}">باسورد جديد</button>
            <button class="btn-red" data-act="delete" data-id="${c.id}" data-name="${esc(c.name)}">حذف</button></td></tr>`).join("")}</table></div>`
        : '<div class="empty">لسه مفيش كافيهات — ضيف أول واحد من فوق.</div>'}
      <pre id="vpnScr" class="hidden" dir="ltr" style="white-space:pre-wrap;background:#0f172a;color:#e2e8f0;padding:12px;border-radius:10px;margin-top:12px"></pre>
      <button class="b-light hidden" id="vpnCopy"><i class="bi bi-clipboard"></i> نسخ الأوامر</button></div>
  </div>`);
  const post = async (body, msg) => { const r = await act(() => api("vpn", { method: "POST", body }), msg); if (r && r.warning) toast(r.warning, true); PAGES.vpn(); };
  $("#vpnSet").onsubmit = e => { e.preventDefault(); const f = formData(e.target); post({ Action: "settings", Host: f.Host, Port: f.Port }, "اتحفظ"); };
  $("#vpnAdd").onsubmit = e => { e.preventDefault(); const f = formData(e.target); post({ Action: "add", Name: f.Name, Phone: f.Phone, ManagerID: +f.ManagerID, Notes: f.Notes }, "الكافيه اتضاف — اطلع أوامر الراوتر"); };
  document.querySelectorAll("[data-act]").forEach(b => b.onclick = () => {
    const a = b.dataset.act;
    if (a === "delete" && !confirm(`حذف الكافيه «${b.dataset.name}»؟ الراوتر بتاعه هيفصل عن الراديوس.`)) return;
    if (a === "new_password" && !confirm("الباسورد القديم هيبطل، ولازم تحط الأوامر الجديدة على راوتر الكافيه. تكمل؟")) return;
    post({ Action: a, ID: +b.dataset.id }, "تم");
  });
  document.querySelectorAll("[data-scr]").forEach(b => b.onclick = async () => {
    const r = await act(() => api("vpn?script=" + b.dataset.scr));
    $("#vpnScr").textContent = r.script; $("#vpnScr").classList.remove("hidden"); $("#vpnCopy").classList.remove("hidden");
    $("#vpnScr").scrollIntoView({ behavior: "smooth" });
  });
  document.querySelectorAll("[data-em]").forEach(b => b.onclick = async () => {
    const r = await act(() => api("cafe?scripts=" + b.dataset.em));
    if (!r) return;
    $("#vpnScr").textContent = "# ⚠️ للعرض بس — ما يتطبقش غير بعد موافقة المالك، وبعد تجربة على روتر تجربة واحد الأول\n" +
      "# ⚠️ الاستهلاك اللي بيحصل محليًا وقت الطوارئ ما بيرجعش للسيستم لسه (الكارت ممكن يستهلك كوتته مرتين)، وملف الكروت لازم يتجدد بإيد كل يوم\n\n" +
      "# ===== 1) إعدادات الهوتسبوت المقترحة =====\n" + r.hotspot +
      "\n# ===== 2) مراقب الطوارئ (مرة واحدة) =====\n" + r.watch +
      "\n# ===== 3) محتوى الملف sr-emergency-users.rsc (" + r.cards + " كارت صالح) — يترفع على Files في الراوتر =====\n" + r.users;
    $("#vpnScr").classList.remove("hidden"); $("#vpnCopy").classList.remove("hidden");
    toast("دي أوامر للعرض بس — ما تتطبقش على الراوتر غير بعد موافقة المالك", true);
    $("#vpnScr").scrollIntoView({ behavior: "smooth" });
  });
  $("#vpnCopy").onclick = () => navigator.clipboard.writeText($("#vpnScr").textContent).then(() => toast("اتنسخ"));
};
