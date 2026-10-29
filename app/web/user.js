// "تعديل المشترك" — copy of ADV's edit page (same sections, field order and column widths: col-md-N → span N of 12).
"use strict";
PAGES.user = async () => {
  // #user?id=N edit · #user?new=1 add (ADV's full add page) · #user?copy=N duplicate (تكرار مشترك)
  const qs = new URLSearchParams(location.hash.split("?")[1] || "");
  const copyOf = qs.get("copy"), creating = !!(qs.get("new") || copyOf);
  const id = creating ? null : qs.get("id");
  const d = creating ? (copyOf ? await api("subscribers/" + copyOf) : { subscriber: { enabled: 1, simul: 1, quota_mb: 0, used_mb: 0 }, invoices: [], radius: [], log: [] }) : await api("subscribers/" + id);
  const s = creating ? { ...d.subscriber, id: null, username: "", online: 0, source: "local", created: "", mac: "", framed_ip: "" } : d.subscriber;
  if (creating) { d.invoices = []; d.radius = []; d.log = []; }
  let ex = {}; try { ex = JSON.parse(s.extra || "{}") || {}; } catch (_) {}
  const v = k => esc(ex[k] ?? "");
  const f = (span, label, inner) => `<div class="fg s${span}"><label>${label}</label>${inner}</div>`;
  const inp = (name, val, attrs = "") => `<input name="${name}" value="${val ?? ""}" ${attrs}>`;
  const sw = (name, on, label) => `<div class="adv-sw"><span>${label}</span><label class="onoff"><input type="checkbox" name="${name}" ${on ? "checked" : ""}><b data-on="ON" data-off="OFF"></b></label></div>`;
  const expVal = (s.expiration || "").replace(" ", "T").slice(0, 16);
  C(`<div class="adv-card box-info">
    <div class="adv-head"><span><i class="fa fa-user"></i> ${creating ? (copyOf ? "تكرار مشترك" : "إضافة مشترك") : `تعديل المشترك (${esc(s.username)})`}</span>
      <span class="head-tags">${+s.online ? '<span class="tag blue">متصل</span>' : ""} ${s.source === "adv" ? '<span class="tag gray">منسوخ من ADV</span>' : ""}</span></div>
    <div class="adv-body">
      <ul class="adv-tabs"><li class="on" data-tab="info">معلومات</li><li data-tab="prices">الأسعار الخاصة</li><li data-tab="history">الفواتير والسجل</li></ul>
      <form id="uf" class="tab-info">
        <h3 class="form-section">معلومات الحساب ${sw("Enabled", +s.enabled, "مفعّل")} ${sw("AutoRenew", +s.auto_renew, "تجديد تلقائى")}</h3>
        <div class="g12">
          ${f(6, "اسم المستخدم", inp("username", esc(s.username), creating ? 'required dir="ltr" autocomplete="off"' : "readonly"))}
          ${f(6, "كَلمة المرور", '<input name="Password" type="password" autocomplete="new-password" placeholder="سيبها فاضية لو مش هتغيرها">')}
          ${f(6, "تأكيد كَلمة المرور", '<input name="Password2" type="password" autocomplete="new-password">')}
          ${f(3, "سرعة التحميل (Kbit/s)", inp("x_down_speed", v("down_speed"), 'dir="ltr"'))}
          ${f(3, "سرعة الرفع (Kbit/s)", inp("x_up_speed", v("up_speed"), 'dir="ltr"'))}
          ${f(2, "سرعة التحميل (Kbit/s)", inp("x_down_speed_temp", v("down_speed_temp"), 'dir="ltr" title="سرعة مؤقتة"'))}
          ${f(2, "سرعة الرفع (Kbit/s)", inp("x_up_speed_temp", v("up_speed_temp"), 'dir="ltr" title="سرعة مؤقتة"'))}
          ${f(2, "الوقت", inp("x_speed_time", v("speed_time")))}
          ${f(2, "&nbsp;", `<select name="x_speed_type">${[["1", "دقائق"], ["2", "ساعات"], ["3", "الأيام"], ["4", "أشهر"]].map(([k, t]) => `<option value="${k}" ${ex.speed_type === k ? "selected" : ""}>${t}</option>`).join("")}</select>`)}
          ${f(6, "الباقة", `<select name="ProfileID">${options(PROFILES, s.profile_id)}</select>`)}
          ${f(3, "المدير", `<select name="ManagerID">${options(MANAGERS, s.manager_id, "username")}</select>`)}
          ${f(3, "عنوان MAC", inp("Mac", esc(s.mac), 'dir="ltr"'))}
          ${f(3, "عنوان IP", inp("FramedIP", esc(s.framed_ip), 'dir="ltr"'))}
          ${f(3, "ايبي الجهاز المرسل", inp("x_ip_add2", v("ip_add2"), 'dir="ltr"'))}
          ${f(3, "معرف الخدمة", inp("x_serv_name", v("serv_name")))}
          ${f(4, "DNS الأساسي (PPP)", inp("x_dns1", v("dns1"), 'dir="ltr"'))}
          ${f(4, "DNS الثانوي (PPP)", inp("x_dns2", v("dns2"), 'dir="ltr"'))}
          ${f(4, "منفذ الاتصال للجهاز", inp("x_portid", v("portid")))}
        </div>
        <h3 class="form-section">معلومات شخصية</h3>
        <div class="g12">
          ${f(6, "الاسم الأول", inp("Firstname", esc(s.firstname)))}
          ${f(6, "الاسم الأخير", inp("Lastname", esc(s.lastname)))}
          ${f(6, "الإيميل", inp("x_email", v("email"), 'dir="ltr"'))}
          ${f(3, "الهاتف", inp("Mobile", esc(s.mobile), 'dir="ltr"'))}
          ${f(3, "InstaPay (مصر فقط)", inp("x_instapay", v("instapay"), 'dir="ltr"'))}
          ${f(4, "الجنسية", inp("x_nationality", v("nationality")))}
          ${f(4, "رقم الهوية", inp("x_national_num", v("national_num")))}
          ${f(4, "طريقة السداد", inp("x_paid_by", v("paid_by")))}
          ${f(6, "الدولة", inp("x_country", v("country") || "EG"))}
          ${f(6, "العنوان", `<textarea name="Address" rows="3">${esc(s.address)}</textarea>`)}
          ${f(6, "ملاحظات", `<textarea name="Notes" rows="3">${esc(String(s.notes || "").replace(/\\n/g, "\n"))}</textarea>`)}
        </div>
        <h3 class="form-section">خصائص متقدمة</h3>
        <div class="g12">
          ${f(6, "وقت الانتهاء", `<input name="Expiration" type="datetime-local" value="${esc(expVal)}">`)}
          ${f(3, "إجمالي وقت الإتصال", inp("x_online_time", v("online_time")))}
          ${f(3, "وقت الإتصال اليومي", inp("x_daily_online_time", v("daily_online_time")))}
          ${f(6, "كمية التحميل المتاحة (MB)", inp("x_in_down", v("in_down")))}
          ${f(6, "كمية الرفع المتاحة (MB)", inp("x_in_up", v("in_up")))}
          ${f(4, "من", inp("x_limit_from_time", v("limit_from_time"), 'type="time"'))}
          ${f(4, "الى", inp("x_limit_to_time", v("limit_to_time"), 'type="time"'))}
          ${f(12, "او قم بتحديد كوتة كلية (MB)", `<div class="adv-inline">${inp("QuotaMB", esc(s.quota_mb), 'type="number" min="0"')}<button type="button" class="b-green" id="qGB" title="تحويل GB إلى MB">GB</button></div>
            <small class="muted">المستخدم منها: ${bytesTxt(s.used_mb)} — المتبقي: ${bytesTxt(s.quota_mb - s.used_mb)}</small>`)}
          ${f(6, "اسم سلسلة التصفية في MikroTik", inp("x_mik_filter", v("mik_filter"), 'dir="ltr"'))}
          ${f(6, "عدد الاجهزة", inp("Simul", esc(s.simul), 'type="number" min="1"'))}
          ${f(6, "MikroTik Address List", inp("x_mik_addr", v("mik_addr"), 'dir="ltr"'))}
          ${f(6, "MikroTik Framed Route", `<textarea name="x_mik_fra" rows="2" dir="ltr">${v("mik_fra")}</textarea>`)}
          ${f(6, "مجموعة مستخدمي MikroTik (Winbox)", inp("x_mik_user", v("mik_user"), 'dir="ltr"'))}
          <div class="fg s6"></div>
          ${f(4, "انشى في", inp("created", advDate(s.created), "readonly"))}
          ${f(4, "انشئ بواسطة", inp("x_creationby", v("creationby") || (s.source === "adv" ? "ADV" : ""), "readonly"))}
          ${f(4, "رقم العقد", inp("x_num_doc", v("num_doc")))}
        </div>
        <div class="form-actions"><button type="button" class="b-cancel" id="uCancel">إلغاء</button><button class="b-save">حفظ</button></div>
      </form>
      <div class="tab-prices hidden"><div id="prTbl"><div class="empty"><i class="fa fa-spinner fa-spin"></i></div></div><p class="muted">الأسعار بتتعدل من صفحة "اسعار العروض" (المدراء ← اسعار العروض).</p></div>
      <div class="tab-history hidden">
        <h3 class="form-section">الفواتير</h3><div class="tablewrap">${table(d.invoices, [["created", "التاريخ"], ["operation", "العملية"], ["amount", "المبلغ"], ["discount", "خصم"], ["paid", "مدفوع", x => +x ? "✔" : "✘"], ["period_to", "لحد"], ["created_by", "بواسطة"]])}</div>
        <h3 class="form-section">إعدادات الراديوس</h3><div class="tablewrap">${table(d.radius, [["t", "النوع"], ["attribute", "الخاصية"], ["op", ""], ["value", "القيمة"]])}</div>
        <h3 class="form-section">سجل العمليات</h3><div class="tablewrap">${table(d.log, [["ts", "الوقت"], ["admin", "بواسطة"], ["action", "العملية"]])}</div>
      </div>
    </div></div>`);
  document.querySelectorAll(".adv-tabs li").forEach(li => li.onclick = () => {
    document.querySelectorAll(".adv-tabs li").forEach(x => x.classList.toggle("on", x === li));
    ["info", "prices", "history"].forEach(t => $(".tab-" + t).classList.toggle("hidden", t !== li.dataset.tab));
    if (li.dataset.tab === "prices" && id) api(`subscribers/${id}/prices`).then(rows => { $("#prTbl").innerHTML = `<div class="tablewrap">${table(rows, [["name", "العرض"], ["base", "السعر الأساسي (EGP)", x => Number(x).toFixed(2)],
      ["price", "سعر المدير (EGP)", (x, r) => `<b class="${+x !== +r.base ? "red" : ""}">${Number(x).toFixed(2)}</b>`]])}</div>`; });
  });
  $("#qGB").onclick = () => { const g = prompt("الكوتة بالجيجا:"); if (g) $("[name=QuotaMB]").value = Math.round(+g * 1024); };
  $("#uCancel").onclick = () => history.back();
  $("#uf").onsubmit = async e => {
    e.preventDefault();
    const fd = formData(e.target), form = e.target;
    if (fd.Password && fd.Password !== fd.Password2) return toast("كلمة المرور والتأكيد مش زي بعض", true);
    const Extra = {}; Object.keys(fd).filter(k => k.startsWith("x_")).forEach(k => Extra[k.slice(2)] = fd[k]);
    const body = { Firstname: fd.Firstname, Lastname: fd.Lastname, Mobile: fd.Mobile, Address: fd.Address, Notes: fd.Notes, Mac: fd.Mac, FramedIP: fd.FramedIP,
      Expiration: fd.Expiration ? fd.Expiration.replace("T", " ") : "", ProfileID: +fd.ProfileID, ManagerID: +fd.ManagerID, Simul: +fd.Simul,
      Enabled: form.Enabled.checked, AutoRenew: form.AutoRenew.checked, Extra, Password: fd.Password || "", QuotaMB: fd.QuotaMB === "" ? null : +fd.QuotaMB };
    if (creating) {
      body.Username = fd.username;
      const r = await act(() => api("subscribers/full", { method: "POST", body }), "اتضاف المشترك");
      location.hash = "user?id=" + r.id;
      return;
    }
    await act(() => api(`subscribers/${id}/full`, { method: "PUT", body }), "تم الحفظ");
  };
};
