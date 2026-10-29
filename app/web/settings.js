// "الإعدادات" — ADV's settings page with the same tabs. Working tabs: إعدادات عامة، متقدم، NAS (expiry pools)، ADV Radius (sync + mode).
"use strict";
const SET_TABS = ["إعدادات عامة", "لوحة تحكم المشترك", "Google Drive", "SMS", "Whatsapp", "بوت واتساب", "Advanced Whatsapp", "FTP/SFTP", "المحفظة الإلكترونية",
  "بوابات الدفع", "إشعارات التطبيق", "تصميم القوالب", "NAS", "متقدم", "رسائل النظام", "ADV Pay", "طابعة الإيصالات", "إدارة الفواتير", "سلة المحذوفات",
  "شهادة SSL", "الترخيص", "ADV Radius", "API للإدارة", "تليجرام", "فاتورة المشترك", "فاتورة المدراء", "صفحة انتهاء الاشتراك", "صفحة فصل الخدمة", "العقد"];
const SET_WORKING = { "إعدادات عامة": 1, "NAS": 1, "متقدم": 1, "ADV Radius": 1, "الترخيص": 1, "SMS": 1, "Whatsapp": 1, "بوت واتساب": 1, "Advanced Whatsapp": 1, "FTP/SFTP": 1, "المحفظة الإلكترونية": 1, "بوابات الدفع": 1, "لوحة تحكم المشترك": 1, "فاتورة المشترك": 1 };
const SET_ICONS = { "إعدادات عامة": "fa-cogs", "لوحة تحكم المشترك": "fa-user", "Google Drive": "fa-google", "SMS": "fa-comment", "Whatsapp": "fa-whatsapp", "بوت واتساب": "fa-android",
  "Advanced Whatsapp": "fa-whatsapp", "FTP/SFTP": "fa-server", "المحفظة الإلكترونية": "fa-credit-card", "بوابات الدفع": "fa-credit-card-alt", "إشعارات التطبيق": "fa-bell", "تصميم القوالب": "fa-paint-brush",
  "NAS": "fa-sitemap", "متقدم": "fa-sliders", "رسائل النظام": "fa-envelope", "ADV Pay": "fa-shopping-cart", "طابعة الإيصالات": "fa-print", "إدارة الفواتير": "fa-file-text-o", "سلة المحذوفات": "fa-trash",
  "شهادة SSL": "fa-lock", "الترخيص": "fa-key", "ADV Radius": "fa-shield", "API للإدارة": "fa-plug", "تليجرام": "fa-paper-plane", "فاتورة المشترك": "fa-file-text", "فاتورة المدراء": "fa-file-text",
  "صفحة انتهاء الاشتراك": "fa-calendar-times-o", "صفحة فصل الخدمة": "fa-ban", "العقد": "fa-file-o" };
PAGES.settings = async () => {
  const d = await api("settings"), S = Object.fromEntries(d.settings.map(x => [x.k, x.v]));
  const tab = decodeURIComponent((location.hash.split("?")[1] || "").replace("tab=", "")) || "إعدادات عامة";
  const fg = (span, label, inner) => `<div class="fg s${span}"><label>${label}</label>${inner}</div>`;
  const inp = (k, def = "", attrs = "") => `<input name="${k}" value="${esc(S[k] ?? def)}" ${attrs}>`;
  const sel = (k, opts, def) => `<select name="${k}">${opts.map(([v, t]) => `<option value="${v}" ${(S[k] ?? def) === v ? "selected" : ""}>${t}</option>`).join("")}</select>`;
  const bodies = {
    "إعدادات عامة": `<div class="g12">${fg(6, "عنوان الموقع", inp("s_title", S.company || "Stream-Net"))}${fg(6, "عملة النظام", sel("currency", [["EGP", "EGP"], ["USD", "USD"], ["SAR", "SAR"]], "EGP"))}
      ${fg(6, "اللغة الافتراضية", sel("def_lang", [["ar", "العربية"], ["en", "English"]], "ar"))}${fg(6, "توقيت بلدك", sel("t_zone", [["Africa/Cairo", "Africa/Cairo"]], "Africa/Cairo"))}
      ${fg(6, "حالة الموقع", sel("site_status", [["1", "مفعّل"], ["0", "مغلق"]], "1"))}${fg(6, "منع المشتركين من تسجيل الدخول", sel("block_login", [["0", "لا"], ["1", "نعم"]], "0"))}
      ${fg(6, "التاريخ والوقت", `<input value="${esc(new Date().toLocaleString("en-GB"))}" readonly>`)}${fg(6, "اسم الشركة", inp("company", "Stream-Net"))}
      ${fg(6, "إيميل الشركة", inp("com_email", "", 'dir="ltr"'))}${fg(6, "الموقع الإلكتروني", inp("website", "", 'dir="ltr"'))}${fg(6, "هاتف الشركة", inp("com_phone", "", 'dir="ltr"'))}
      ${fg(6, "الدولة", sel("country", [["EG", "مصر"]], "EG"))}${fg(12, "العنوان", inp("address"))}</div>`,
    "NAS": `<div class="g12">${fg(6, "Pool المنتهي اشتراكهم (زي END)", inp("expired_pool", "END", 'dir="ltr"'))}${fg(6, "Pool اللي خلصت كوتتهم (زي LIMIT)", inp("quota_pool", "LIMIT", 'dir="ltr"'))}
      ${fg(6, "سرعة المنتهي (اختياري، مثلا 256k/256k)", inp("expired_rate", "", 'dir="ltr"'))}${fg(6, "الكوتة بتتحسب من (بعد النقل)", sel("quota_count", [["both", "التحميل + الرفع"], ["down", "التحميل بس"], ["up", "الرفع بس"]], "both"))}${fg(6, "ربط اليوزر بالـ MAC", sel("mac_bind", [["0", "مقفول"], ["1", "مفعّل"]], "0"))}</div>` + specForm(SET_SPEC["NAS_extra"], S),
    "متقدم": `<div class="g12">${fg(6, "ينتهي قريب = خلال كام يوم", inp("expiring_days", "3"))}${fg(6, "أيام الباقة الافتراضية", inp("default_days", "30"))}
      ${fg(6, "كلمة سر PPPoE الموحدة", `<input value="${d.default_password_set ? "متسجلة في ملف الإعدادات على السيرفر" : "مش متسجلة!"}" readonly>`)}</div>` + specForm(SET_SPEC["متقدم_extra"], S),
    "ADV Radius": `<div class="g12">${fg(6, "المزامنة الكاملة مع ADV كل كام دقيقة", inp("sync_minutes", "15"))}${fg(6, "المزامنة السريعة (التغييرات بس) كل كام ثانية", inp("fast_seconds", "20"))}${fg(6, "الوضع الحالي", `<input value="${S.mode === "shadow" ? "مراقبة (Shadow) — ADV هو الشغال" : "تشغيل (Live)"}" readonly>`)}</div>
      <p class="muted">التحويل لـ Live بيفتح التجديد والفلوس هنا، ومش بيغيّر أي حاجة على السيرفرات. ربط السيرفرات بالراديوس الجديد خطوة منفصلة بموافقة المالك.</p>
      <button type="button" class="${S.mode === "shadow" ? "btn-red" : "btn-orange"}" id="modeBtn">${S.mode === "shadow" ? "حوّل لـ Live" : "رجّع لـ Shadow"}</button>`,
    "SMS": '<div id="smsPanel"></div>' + specForm(SET_SPEC["SMS"], S), "Whatsapp": `<div id="waPanel"></div><h4 class="form-section">تذكير المديونية والنسخة الاحتياطية</h4><div class="g12">${fg(12, "رسالة تذكير المديونية (فاضية = مقفولة) — المتغيرات: {name} {user} {debt} {company}", `<textarea name="adv_set_debt_msg_whats" rows="3">${esc(S.adv_set_debt_msg_whats || "")}</textarea>`)}${fg(4, "التذكير كل كام يوم", inp("debt_rem_days", "3", 'type="number" min="1" max="30"'))}${fg(4, "أقل مديونية للتذكير (جنيه)", inp("debt_rem_min", "1", 'type="number" min="1"'))}${fg(4, "رقم يستلم النسخة الاحتياطية كل يوم (فاضي = مقفول)", inp("backup_wa_to", "", 'dir="ltr" placeholder="01xxxxxxxxx"'))}</div><p class="muted">النسخة الاحتياطية فيها بيانات كل المشتركين — ابعتها لرقمك إنت بس. ${esc(S.backup_wa_state || "")}</p><button type="button" class="b-light" id="bkWa"><i class="bi bi-whatsapp"></i> ابعت آخر نسخة دلوقتي</button><div class="g12"><div class="fg s6"><label>رقم واتساب الشركة</label><input name="wa_number" dir="ltr" value="${esc(S.wa_number || "")}"></div><div class="fg s6"><label>ثواني بين كل رسالة</label><input name="wa_gap" type="number" min="3" value="${esc(S.wa_gap || 8)}"></div></div>
      <p class="muted">المتغيرات في الرسايل: {user} {name} {prof} {exp} {price} {money} {days} {company} — الرسايل التلقائية بتشتغل بس في وضع التشغيل (Live)، في وضع المراقبة ADV هو اللي بيبعت.</p>` + specForm(SET_SPEC["Whatsapp"], S), "بوت واتساب": specForm(SET_SPEC["بوت واتساب"], S), "Advanced Whatsapp": specForm(SET_SPEC["Advanced Whatsapp"], S),
    "FTP/SFTP": specForm(SET_SPEC["FTP/SFTP"], S), "المحفظة الإلكترونية": specForm(SET_SPEC["المحفظة الإلكترونية"], S), "بوابات الدفع": specForm(SET_SPEC["بوابات الدفع"], S), "لوحة تحكم المشترك": `<div class="hint-box"><i class="bi bi-person-workspace"></i> بوابة المشترك: <a href="/portal" target="_blank" dir="ltr">${esc(location.origin)}/portal</a> — وصفحة الخدمة المتوقفة: <a href="/expired" target="_blank" dir="ltr">${esc(location.origin)}/expired</a> (وجّه الـ END/LIMIT ليها من الميكروتيك).</div><div class="g12">${fg(6, "البوابة", sel("portal_on", [["1", "مفتوحة"], ["0", "مقفولة"]], "1"))}${fg(6, "الدخول برقم الموبايل", sel("portal_phone_login", [["1", "مسموح"], ["0", "لا"]], "1"))}${fg(4, "التجديد من الرصيد", sel("portal_allow_renew", [["1", "مسموح"], ["0", "لا"]], "1"))}${fg(4, "السلفة (للمنتهي، يومين)", sel("portal_allow_advance", [["1", "مسموحة"], ["0", "لا"]], "1"))}${fg(4, "تغيير كلمة السر", sel("portal_allow_password", [["1", "مسموح"], ["0", "لا"]], "1"))}${fg(12, "إعلان يظهر للمشتركين (اختياري)", inp("portal_announce"))}</div>` + specForm(SET_SPEC["لوحة تحكم المشترك"], S),
    "فاتورة المشترك": `<div class="g12">${fg(12, "شكل إيصال المشترك (HTML) — فاضي = الشكل الافتراضي", `<textarea name="adv_set_invoice_tpl" rows="12" dir="ltr">${esc(S.adv_set_invoice_tpl || "")}</textarea>`)}</div>
      <p class="muted">المتغيرات: {company} {user} {name} {prof} {amount} {from} {to} {date} {by} {mobile} {notes}</p>`,
    "الترخيص": `<div class="kv"><div>النظام</div><div>RAD-Manager ${esc(d.host)} — ملكك، من غير ترخيص ولا حد أقصى للمشتركين</div><div>ADV الحالي</div><div>مرخّص لـ 4000 مشترك بس (حسب تنبيه لوحة ADV)</div></div>`,
  };
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-cog"></i> الإعدادات</span></div><div class="adv-body set-wrap">
    <div class="set-tabs">${SET_TABS.map(t => `<a data-tab="${esc(t)}" class="${t === tab ? "on" : ""} ${SET_WORKING[t] ? "" : "soon"}"><i class="fa ${SET_ICONS[t] || "fa-cog"}"></i>${esc(t)}</a>`).join("")}</div>
    <form id="sf" class="set-body"><h3 class="form-section">${esc(tab)}</h3>${bodies[tab] || '<div class="empty">التاب ده هيتعمل في المرحلة الجاية</div>'}
      ${bodies[tab] && tab !== "الترخيص" ? '<div class="form-actions"><button class="b-save">حفظ</button></div>' : ""}</form></div></div>`);
  document.querySelectorAll("[data-tab]").forEach(a => a.onclick = () => { location.hash = "settings?tab=" + encodeURIComponent(a.dataset.tab); });
  if ($("#waPanel")) waPanel($("#waPanel"));
  if ($("#bkWa")) $("#bkWa").onclick = () => act(() => api("backups/whatsapp", { method: "POST", body: {} }), "اتبعتت على الواتساب");
  if ($("#smsPanel")) smsPanel($("#smsPanel"), S);
  $("#sf").onsubmit = async e => { e.preventDefault(); await act(() => api("settings", { method: "PUT", body: specSubmit(e.target) }), "تم الحفظ"); };
  const mb = $("#modeBtn");
  if (mb) mb.onclick = async () => {
    const to = S.mode === "shadow" ? "live" : "shadow";
    const c = prompt(`علشان تحوّل لـ ${to} اكتب: نعم انقل`); if (c !== "نعم انقل") return;
    await act(() => api("settings", { method: "PUT", body: { mode: to, confirm: c } }), "اتغير"); location.reload();
  };
};
