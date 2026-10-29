// The remaining ADV settings tabs, with ADV's own field names (saved as settings "adv_set_<name>"); gateways get wired in the next phase.
"use strict";
const T = (n, l) => [n, l, "text"], P = (n, l) => [n, l, "password"], A = (n, l) => [n, l, "textarea"], CB = (n, l) => [n, l, "checkbox"], N = (n, l) => [n, l, "number"], SEL = (n, l, o) => [n, l, "select", o];
const MSGS = (sfx, extra = []) => [T("api_url" + sfx, "رابط الـ API"), ...extra, A("create" + sfx + "_msg", "رسالة إنشاء مشترك جديد"), A("act" + sfx + "_msg", "رسالة التفعيل"), A("act_msg_ch" + sfx, "رسالة تفعيل السُلفة"),
  A("act_msg_auto_renew" + sfx, "رسالة التفعيل التلقائي"), A("msg_q" + sfx, "رسالة إضافة كوتة"), A("ch_prof_msg" + sfx, "رسالة تغيير الباقة للمشترك"), A("dep_man_msg" + sfx, "رسالة إيداع رصيد للمدير"),
  A("withd_man_msg" + sfx, "رسالة سحب رصيد من المدير"), A("exp_msg" + sfx, "رسالة ما قبل انتهاء الاشتراك"), SEL("exp_rem" + sfx, "إرسال رسالة انتهاء الاشتراك قبل", ["1 الأيام", "2 الأيام", "3 الأيام", "4 الأيام", "5 الأيام"]),
  A("expire_msg" + sfx, "رسالة انتهاء الاشتراك"), A("dep_user_msg" + sfx, "رسالة إيداع رصيد للمشترك"), A("withd_user_msg" + sfx, "رسالة سحب رصيد من المشترك"), A("part_inv_msg" + sfx, "رسالة سداد جزئي للمدير"),
  A("part_inv_user_msg" + sfx, "رسالة سداد جزئي للمشترك"), A("trans_inv_user_msg" + sfx, "رسالة تحويل رصيد لمشترك"), A("receive_inv_user_msg" + sfx, "رسالة استلام رصيد من مشترك"),
  A("quota_usage_msg" + sfx, "رسالة استهلاك الجيجابايتات عند الوصول"), SEL("quota_usage_pct" + sfx, "ابعتها لما الاستهلاك يوصل", ["70 %", "50 %", "80 %", "90 %"]), A("recharge_end_msg" + sfx, "رسالة نفاد الباقة الإضافية"), A("msg_now" + sfx, "رسالة فورية لكل المستخدمين")];
const SET_SPEC = {
  "SMS": MSGS("_sms", [T("api_balance", "API Balance")]),
  "Whatsapp": [P("wa_official_token", "Access Token (Permanent)"), T("wa_official_phone_id", "Phone Number ID"), T("wa_official_business_id", "Business Account ID"), T("wa_webhook_url", "Webhook Callback URL"),
    T("wa_official_verify_token", "Webhook Verify Token"), T("wa_official_app_id", "Meta App ID (اختياري)"), P("wa_official_app_secret", "Meta App Secret (اختياري)"), ...MSGS("_whats")],
  "بوت واتساب": [CB("bot_enabled", "تفعيل البوت"), T("trigger_word", "بادئة البوت"), T("signature", "التوقيع"), T("unregistered_message", "رسالة لغير المسجَّلين"), T("bot_start_time", "وقت بدء التشغيل"), T("bot_end_time", "وقت إغلاق البوت"),
    T("bot_api_url", "رابط API الواتساب (للبوت)"), A("excluded_numbers", "أرقام مستثناة (لا تستخدم البوت)")],
  "Advanced Whatsapp": [T("api_url_bup_advwt", "API URL (Files)"), SEL("backup_wt_time", "تكرار النسخ", ["1", "2", "3", "4", "5"]), SEL("val_typewt_backup", "الوحدة", ["ساعات", "الأيام", "أشهر", "أعوام"]), T("api_url_advwt", "رابط الـ API"),
    A("noti_cr_msg_wt", "عند إنشاء مشترك جديد"), A("noti_act_wt_msg", "رسالة التفعيل"), A("act_msg_wt", "رسالة تفعيل السُلفة"), A("msg_q_wt", "رسالة إضافة كوتة"), A("dep_user_msg_wt", "رسالة إيداع رصيد للمشترك"),
    A("withd_user_msg_wt", "رسالة سحب رصيد من المشترك"), A("ch_prof_msg_wt", "رسالة تغيير الباقة للمشترك"), A("part_inv_user_msg_wt", "رسالة سداد جزئي للمشترك"), A("dep_man_msg_wt", "رسالة إيداع رصيد للمدير"),
    A("withd_man_msg_wt", "رسالة سحب رصيد من المدير"), A("part_inv_msg_wt", "رسالة سداد جزئي للمدير")],
  "FTP/SFTP": [T("ftp_server", "السيرفر:"), N("ftp_port", "البورت:"), T("ftp_username", "اسم المستخدم:"), P("ftp_password", "كلمة المرور:"), SEL("ftp_connectionType", "نوع الاتصال:", ["FTP", "SFTP"]), T("ftp_serverPath", "المسار على السيرفر:"),
    N("ftp_backupValue", "تكرار النسخ الاحتياطي كل:"), SEL("ftp_backupUnit", "الوحدة الزمنية:", ["ساعة", "ساعات", "يوم", "أيام", "أسبوع", "أسابيع"])],
  "المحفظة الإلكترونية": [CB("enable_wallet", "تفعيل الدفع"), CB("charge_commission", "خصم العمولة من المستخدم"), CB("charge_admin", "تفعيل الشحن للموزّع"), CB("dis_trans", "تعطيل رقم المعاملة لغير المسجَّلين"), CB("send_trans_unreg", "إرسال رقم معاملة لغير المسجَّلين"),
    CB("enable_contact_card", "إتاحة إنشاء كارت اتصال"), CB("enable_recharge_card", "إتاحة إنشاء كارت شحن"), P("api_key_wallet", "مفتاح الدفع"), A("wallet_explanation", "شرح عملية التحويل"), A("transaction_message", "رسالة المعاملة (SMS)"),
    A("transaction_card_message", "رسالة كروت الاتصال (SMS)"), A("transaction_card_ch_message", "رسالة كروت الشحن (SMS)"), A("balance_add_message", "رسالة إضافة رصيد للمشترك (SMS)"), A("balance_add_message_ad", "رسالة إضافة رصيد للمدير (SMS)"),
    CB("ai_enabled", "تفعيل الذكاء الاصطناعي"), SEL("ai_provider", "مزوّد الذكاء الاصطناعي", ["Google Gemini (مجاني)", "Groq (مجاني)", "Grok - xAI (مدفوع)", "OpenAI (مدفوع)", "Claude (مدفوع)"]), P("ai_api_key", "مفتاح API"), A("custom_regex", "أنماط Regex (كل نمط في سطر)"), A("test_message", "رسالة اختبار")],
  "بوابات الدفع": [CB("enable_payment", "تفعيل بوّابات الدفع"), SEL("payment_currency", "العملة", ["جنيه مصري (EGP)", "دولار أمريكي (USD)", "يورو (EUR)", "ريال سعودي (SAR)", "دينار ليبي (LYD)"]), SEL("payment_commission_type", "نوع العمولة", ["نسبة مئوية %", "مبلغ ثابت"]),
    N("payment_commission_value", "قيمة العمولة"), N("min_amount", "الحد الأدنى"), N("max_amount", "الحد الأقصى"), CB("pay_recharge", "شحن رصيد المشتركين"), CB("pay_renew", "تجديد الاشتراك"), CB("pay_cards", "شراء كروت أونلاين"),
    SEL("paypal_mode", "PayPal — الوضع", ["Sandbox (تجريبي)", "Live (حقيقي)"]), T("paypal_client_id", "Client ID"), P("paypal_secret", "Secret Key"), SEL("stripe_mode", "Stripe — الوضع", ["Test (تجريبي)", "Live (حقيقي)"]), T("stripe_publishable_key", "Publishable Key"),
    P("stripe_secret_key", "Secret Key"), P("stripe_webhook_secret", "Webhook Secret"), P("pgw_paymob_s_api_key", "Paymob — مفتاح API"), T("pgw_paymob_s_integration_id", "معرّف التكامل (Integration ID)"), T("pgw_paymob_s_iframe_id", "معرّف الإطار (iFrame ID)"),
    P("pgw_paymob_s_hmac", "مفتاح HMAC"), T("pgw_fawry_s_merchant_code", "فوري — كود التاجر"), P("pgw_fawry_s_security_key", "مفتاح الأمان"), T("pgw_fawry_s_user_identifier", "معرّف مستخدم فوري"), P("pgw_fawry_s_password", "كلمة المرور")],
  "لوحة تحكم المشترك": [T("count_num", "عدد أرقام الهاتف"), SEL("ucp_access_mode", "طريقة تشغيل لوحة المشترك", ["ضمن رابط النظام", "على عنوان مستقل", "معطلة"]), T("ucp_public_url", "الرابط العام للوحة المشترك"),
    CB("ucp_internet_control", "السماح للمشترك بالتحكم بالإنترنت (إيقاف/تشغيل)"), T("ucp_full_url", "رابط دخول المشتركين"), CB("bc_active", "تفعيل رسالة البث"), SEL("bc_type", "نوع الرسالة", ["معلومة", "تحذير", "هام", "بشرى"]), CB("bc_auto_close", "الإغلاق"),
    T("bc_title", "عنوان الرسالة"), A("bc_message", "نص الرسالة")],
  "NAS_extra": [T("port_incoming", "Port Incoming"), T("default_ftp_port", "بورت FTP الافتراضي")],
  "متقدم_extra": [T("idle_timeout", "NAS مهلة تعطيل"), SEL("idle_type", "الوحدة", ["دقائق", "ساعات", "الأيام", "أشهر"]), T("auto_del", "حذف المنتهي تلقائيا (مشتركين)"), SEL("del_users_ch", "الوحدة", ["دقائق", "ساعات", "الأيام", "أشهر", "أعوام"]),
    T("auto_dis_user", "تعطيل المنتهي تلقائيا (مشتركين)"), SEL("dis_user_ch", "الوحدة", ["دقائق", "ساعات", "الأيام", "أشهر", "أعوام"]), N("auto_rem_radpost", "حذف سجلات المصادقة تلقائياً (أيام)"),
    SEL("auto_kick_mode", "طرد الجلسة القديمة عند تسجيل دخول جديد", ["معطّل", "الكروت فقط", "المشتركون فقط", "الكل"]), T("dis_pool_name", "Pool Name (المعطّل)"), T("notfound_pool_name", "Pool Name (غير موجود)"),
    T("num_try_ch", "عدد محاولات إدخال الكارت"), T("time_try_ch", "تحديد وقت للحظر (دقائق)"), T("less_price", "حد أدنى للمبلغ"), T("large_price", "حد أقصى للمبلغ"), T("less_price_g", "حد أدنى للأيام"), T("large_price_g", "حد أقصى للأيام"),
    CB("refuse_mac", "رفض الـ MAC العشوائي (مشتركين)"), CB("hide_quota_charge", "إخفاء الكوتة المتبقية من شاشة الشحن"), CB("refuse_mac_cards", "رفض الـ MAC العشوائي (كروت)"), T("new_mac", "إضافة عنوان MAC موثوق:")],
};
// render a spec as an ADV-style form; values live in settings as adv_set_<name>
function specForm(spec, S) {
  const v = n => S["adv_set_" + n] ?? "";
  return `<div class="g12">${spec.map(([n, l, t, o]) => { const span = t === "textarea" ? 12 : 6;
    const inner = t === "textarea" ? `<textarea name="adv_set_${n}" rows="2">${esc(v(n))}</textarea>` : t === "checkbox" ? `<label class="onoff"><input type="checkbox" name="adv_set_${n}" ${v(n) === "1" ? "checked" : ""}><b data-on="ON" data-off="OFF"></b></label>`
      : t === "select" ? `<select name="adv_set_${n}">${o.map(x => `<option ${v(n) === x ? "selected" : ""}>${esc(x)}</option>`).join("")}</select>` : `<input name="adv_set_${n}" type="${t}" value="${esc(v(n))}" ${t === "password" ? 'autocomplete="new-password"' : ""} dir="${t === "text" && /url|api|id|key|token|path|server/i.test(n) ? "ltr" : "auto"}">`;
    return `<div class="fg s${span}"><label>${esc(l)}</label>${inner}</div>`; }).join("")}</div>`;
}
function specSubmit(form) { const b = {}; [...form.elements].forEach(e => { if (!e.name) return; b[e.name] = e.type === "checkbox" ? (e.checked ? "1" : "0") : e.value; }); return b; }
