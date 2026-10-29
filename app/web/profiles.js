// "العروض" — ADV's profile list and add/edit page (adv-docs/ADV-PROFILES.md).
"use strict";
const mbps = k => { const n = Number(k || 0); if (!n) return "غير محدود"; return n >= 1024 ? `${+(n / 1024).toFixed(2)} Mbps` : `${n} Kbps`; };
const gbTxt = mb => { const n = Number(mb || 0); if (!n) return "غير محدود"; return n >= 1024 ? `${+(n / 1024).toFixed(2)} GB` : `${n} MB`; };
const EXP_UNIT = { "1": "ساعات", "2": "الأيام", "3": "أشهر", "4": "دقائق" };

PAGES.profiles = async () => {
  const list = await api("profiles/full");
  PROFILES = list;
  const byAdv = Object.fromEntries(list.map(p => [p.adv_id, p.name]));
  const next = v => !v || v === "0" ? "لا شيء" : (byAdv[v] || v);
  const sel = new Set();
  let size = 25, page = 1, q = "";
  C(`<div class="adv-card">
    <div class="adv-head"><span><i class="fa fa-tags"></i> قائمة العروض</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div>
    <div class="adv-body">
      <div class="adv-actions" style="margin:0 0 12px">
        <button class="btn-hoki" id="pAdd"><i class="fa fa-plus"></i> إضافة</button>
        <button class="btn-blue2" id="pEdit"><i class="fa fa-edit"></i> تعديل</button>
        <button class="btn-hoki" id="pCopy"><i class="fa fa-copy"></i> نسخ</button>
        <button class="btn-red" id="pDel"><i class="fa fa-trash"></i> حذف</button>
        <button class="btn-hoki" id="pMsg"><i class="fa fa-envelope"></i> إرسال رسالة</button>
        <span class="dd"><button class="b-gray btn-chambray" data-dd="pMore">المزيد <i class="fa fa-caret-down"></i></button><div class="dd-menu" id="pMore">
          <a class="soon">تحديد فترات باندويث</a><a class="soon">تحديد محاسبة خاصة</a><a class="soon">تحديد سرعات متعددة</a></div></span>
      </div>
      <div class="adv-tools"><label>إظهار <select id="psz">${[10, 25, 50, 100, 200, 500].map(n => `<option value="${n}" ${n === size ? "selected" : ""}>${n} في الصفحة</option>`).join("")}</select> نتائج</label>
        <label>بحث: <input id="pq"></label></div>
      <div class="tablewrap"><table class="adv-table" id="pT"></table></div>
      <div class="adv-tools"><div id="pInfo" class="muted"></div><div class="adv-pager" id="pPg"></div></div>
    </div></div>`);
  const draw = () => {
    const rows = list.filter(p => !q || p.name.toLowerCase().includes(q));
    const pages = Math.max(1, Math.ceil(rows.length / size)); page = Math.min(page, pages);
    const view = rows.slice((page - 1) * size, page * size);
    $("#pT").innerHTML = `<thead><tr><th class="c-chk"><input type="checkbox" id="pAll"></th><th>#</th><th>الاسم</th><th>السعر (EGP)</th><th>سرعة التحميل</th><th>سرعة الرفع</th>
      <th>وقت الانتهاء</th><th>كمية التحميل</th><th>كمية الرفع</th><th>الكوتة الكلية</th><th>IP Pool</th><th>عدد المشتركين</th><th>عند انتهاء الكوتة</th>
      <th>عند انتهاء الكوتة اليومية</th><th>عند انتهاء الوقت</th></tr></thead><tbody>${view.map((p, i) => {
        const f = p.f || {}, sys = /^(END|LIMIT)$/i.test(p.name) || !+p.enabled;
        const unit = EXP_UNIT[f.exp_unit_val] || "الأيام", n = f.exp_unit || p.days;
        return `<tr><td class="c-chk"><input type="checkbox" data-p="${p.id}" ${sel.has(p.id) ? "checked" : ""}></td>
        <td class="c-n" style="background:${sys ? "rgba(255,59,59,.88)" : "#2dd22d"}">${(page - 1) * size + i + 1}</td>
        <td><a class="plink" data-edit="${p.id}">${esc(p.name)}</a></td><td>${Number(p.price).toFixed(2)}</td><td>${mbps(p.down_speed)}</td><td>${mbps(p.up_speed)}</td>
        <td>${+f.type_prof === 1 || sys ? "" : `${esc(n)} ${unit}`}</td><td>${+f.profile_down_qouta ? gbTxt(f.profile_down_qouta) : "--"}</td><td>${+f.profile_up_qouta ? gbTxt(f.profile_up_qouta) : "--"}</td>
        <td>${gbTxt(p.quota_mb)}</td><td>${esc(p.pool || f.pool_name || "")}</td><td>${p.users}</td><td>${esc(next(f.qouta_expire_service))}</td>
        <td>${esc(next(f.daily_expire_service))}</td><td>${esc(f.profile_expire_service === "0" || !f.profile_expire_service ? "إنهاء الاشتراك" : next(f.profile_expire_service))}</td></tr>`;
      }).join("") || '<tr><td colspan="15" class="empty">لا توجد بيانات</td></tr>'}</tbody>`;
    $("#pInfo").textContent = `إظهار ${rows.length ? (page - 1) * size + 1 : 0} إلى ${Math.min(rows.length, page * size)} من أصل ${rows.length} مُدخل`;
    $("#pPg").innerHTML = `<a class="${page <= 1 ? "dis" : ""}" data-pg="${page - 1}">السابقة</a>` + Array.from({ length: pages }, (_, i) => `<a class="${i + 1 === page ? "cur" : ""}" data-pg="${i + 1}">${i + 1}</a>`).join("") + `<a class="${page >= pages ? "dis" : ""}" data-pg="${page + 1}">التالية</a>`;
    $("#pPg").querySelectorAll("[data-pg]").forEach(a => a.onclick = () => { const n = +a.dataset.pg; if (n >= 1 && n <= pages) { page = n; draw(); } });
    document.querySelectorAll("[data-p]").forEach(c => c.onchange = () => c.checked ? sel.add(+c.dataset.p) : sel.delete(+c.dataset.p));
    $("#pAll").onchange = e => { view.forEach(p => e.target.checked ? sel.add(p.id) : sel.delete(p.id)); draw(); };
    document.querySelectorAll("[data-edit]").forEach(a => a.onclick = () => { location.hash = "profile?id=" + a.dataset.edit; });
  };
  $("#psz").onchange = e => { size = +e.target.value; page = 1; draw(); };
  $("#pq").oninput = e => { q = e.target.value.trim().toLowerCase(); page = 1; draw(); };
  $("#reload").onclick = () => PAGES.profiles();
  const one = () => { if (sel.size !== 1) { toast("اختار عرض واحد", true); return null; } return [...sel][0]; };
  $("#pAdd").onclick = () => { location.hash = "profile"; };
  $("#pEdit").onclick = () => { const id = one(); if (id) location.hash = "profile?id=" + id; };
  $("#pCopy").onclick = () => { const id = one(); if (id) location.hash = "profile?copy=" + id; };
  $("#pDel").onclick = () => toast("الحذف مقفول: ما ينفعش تمسح عرض عليه مشتركين من هنا", true);
  $("#pMsg").onclick = () => toast("إرسال الرسائل هيتعمل في المرحلة الجاية", true);
  document.querySelectorAll("[data-dd]").forEach(b => b.onclick = e => { e.stopPropagation(); $("#" + b.dataset.dd).classList.toggle("open"); });
  document.querySelectorAll("#pMore a").forEach(a => a.onclick = () => toast("هتتعمل في المرحلة الجاية", true));
  draw();
};

PAGES.profile = async () => {
  const qs = new URLSearchParams(location.hash.split("?")[1] || "");
  const list = await api("profiles/full"); PROFILES = list;
  const src = list.find(p => p.id === +(qs.get("id") || qs.get("copy")));
  const editing = !!qs.get("id") && src;
  const f = Object.assign({}, src ? src.f : {}, src ? { profile_name: (qs.get("copy") ? src.name + " - نسخة" : src.name), price: src.price, down_speed: src.down_speed,
    up_speed: src.up_speed, profile_qouta: src.quota_mb, pool_name: src.pool || (src.f || {}).pool_name } : { exp_unit: 30, exp_unit_val: "2" });
  const v = k => esc(f[k] ?? "");
  const fg = (span, label, inner) => `<div class="fg s${span}"><label>${label}</label>${inner}</div>`;
  const inp = (k, attrs = "") => `<input name="${k}" value="${v(k)}" ${attrs}>`;
  const svc = (k, first) => `<select name="${k}"><option value="0">${first}</option>${list.map(p => `<option value="${esc(p.adv_id || p.id)}" ${String(f[k]) === String(p.adv_id || p.id) ? "selected" : ""}>${esc(p.name)}</option>`).join("")}</select>`;
  const units = (k, opts) => `<select name="${k}">${opts.map(([val, t]) => `<option value="${val}" ${String(f[k]) === val ? "selected" : ""}>${t}</option>`).join("")}</select>`;
  C(`<div class="adv-card box-info"><div class="adv-head"><span><i class="fa fa-tag"></i> ${editing ? "تعديل العرض (" + esc(src.name) + ")" : "إضافة عرض جديد"}</span></div>
    <div class="adv-body"><form id="pf">
      <h3 class="form-section">إعدادات عامة</h3><div class="g12">
        ${fg(6, "اسم العرض", inp("profile_name", "required"))}${fg(6, "السعر (EGP)", inp("price", 'type="number" step="0.01"'))}
        ${fg(6, "وحدات تجريبية للاختبار (24 ساعة)", inp("test_units"))}<div class="fg s6"></div>
        ${fg(4, "سرعة التحميل (Kbit/s)", inp("down_speed", 'dir="ltr"'))}${fg(4, "سرعة الرفع (Kbit/s)", inp("up_speed", 'dir="ltr"'))}<div class="fg s4"></div>
        ${fg(6, "CIR للتنزيل (Kbit/s)", inp("down_cir"))}${fg(6, "CIR للرفع (Kbit/s)", inp("up_cir"))}
        ${fg(6, "سياسة التنزيل (Cisco)", inp("down_policy", 'dir="ltr"'))}${fg(6, "سياسة الرفع (Cisco)", inp("up_policy", 'dir="ltr"'))}
        ${fg(4, "DNS الأساسي (PPP)", inp("dns1", 'dir="ltr"'))}${fg(4, "DNS الثانوي (PPP)", inp("dns2", 'dir="ltr"'))}${fg(4, "منفذ الاتصال للجهاز", inp("portid"))}
        ${fg(3, "ينتهي بعد", inp("exp_unit", 'type="number" min="0"'))}${fg(3, "&nbsp;", units("exp_unit_val", [["3", "أشهر"], ["2", "الأيام"], ["1", "ساعات"], ["4", "دقائق"]]))}
      </div>
      <h3 class="form-section">إعدادات الكوتة</h3><div class="g12">
        ${fg(6, "كمية التحميل للعرض (MB)", inp("profile_down_qouta"))}${fg(6, "كمية الرفع للعرض (MB)", inp("profile_up_qouta"))}
        ${fg(12, "او قم بتحديد كوتة كلية (MB)", inp("profile_qouta"))}
        ${fg(6, "كمية التحميل اليومي (MB)", inp("daily_down_qouta"))}${fg(6, "كمية الرفع اليومي (MB)", inp("daily_up_qouta"))}
        ${fg(12, "او قم بتحديد كوتة يومية (MB)", inp("daily_profile_qouta"))}
      </div>
      <h3 class="form-section">🎁 إعدادات العرض المؤقت</h3><div class="g12">
        ${fg(3, "الكوتة (MB)", inp("qouta_give_money"))}${fg(3, "الوقت (ساعات)", inp("time_give_money"))}${fg(4, "عدد مرات الإستلاف (مديرين)", inp("num_give_money"))}<div class="fg s2"></div>
        ${fg(4, "وقت الإتصال بالانترنت", inp("online_time"))}${fg(2, "&nbsp;", units("hours_min", [["1", "دقائق"], ["2", "ساعات"]]))}
        ${fg(4, "وقت الإتصال اليومي", inp("daily_online_time"))}<div class="fg s2"></div>
      </div>
      <h3 class="form-section">إعدادات متقدمة</h3><div class="g12">
        ${fg(6, "Pool Name", inp("pool_name", 'dir="ltr"'))}${fg(6, "تفعيل البيرست", units("enable_burst", [["0", "لا"], ["1", "نعم"]]))}
        ${fg(6, "حد البيرست للتنزيل (Kbps)", inp("burst_down_limit"))}${fg(6, "حد البيرست للرفع (Kbps)", inp("burst_up_limit"))}
        ${fg(6, "حد البيرست للتنزيل (Kbps)", inp("burst_down_thr"))}${fg(6, "حد البيرست للرفع (Kbps)", inp("burst_up_thr"))}
        ${fg(6, "وقت البيرست للتنزيل (ثانية)", inp("burst_down_time"))}${fg(6, "وقت البيرست للرفع (ثانية)", inp("burst_up_time"))}
        ${fg(6, "عند انتهاء الكوتة", svc("qouta_expire_service", "لا شيء"))}${fg(3, "تحديد نسب مئوية لإرسال الإشعارات", inp("percentages", 'dir="ltr"'))}
        ${fg(3, "عند انتهاء الكوتة اليومية", svc("daily_expire_service", "لا شيء"))}
        ${fg(12, "عند انتهاء الوقت", svc("profile_expire_service", "إنهاء الاشتراك"))}
        ${fg(6, "MTU", inp("v_mtu"))}${fg(6, "أدخل اليوم", inp("set_day_end"))}${fg(5, "أدخل الساعة", inp("set_hour_end"))}<div class="fg s7"></div>
        ${fg(6, "اسم سلسلة الفلترة في MikroTik", inp("mik_filter", 'dir="ltr"'))}${fg(6, "MikroTik Address List", inp("mik_addr", 'dir="ltr"'))}
        ${fg(6, "MikroTik Queue Priority (Simple Queue)", inp("mik_queue", 'dir="ltr"'))}${fg(6, "مجموعة المستخدمين في MikroTik", inp("mik_user", 'dir="ltr"'))}
      </div>
      <div class="form-actions"><button type="button" class="b-cancel" id="pCancel">إلغاء</button><button class="b-save">حفظ</button></div>
    </form></div></div>`);
  $("#pCancel").onclick = () => { location.hash = "profiles"; };
  $("#pf").onsubmit = async e => {
    e.preventDefault();
    const body = formData(e.target);
    await act(() => api(editing ? `profiles/${src.id}/full` : "profiles/full", { method: editing ? "PUT" : "POST", body }), "تم الحفظ");
    location.hash = "profiles";
  };
};
