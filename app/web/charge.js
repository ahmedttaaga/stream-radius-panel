// "لوحة الشحن" — ADV's quick renew screen: find a subscriber, pick the activation type, press تفعيل.
"use strict";
PAGES.charge = async () => {
  const d = await api("dashboard");
  let qs = {};
  if (d.mode === "shadow") { try { qs = ((await api("sync")).adv_metrics || {}).quick_stats || {}; } catch (_) {} }
  const today = d.mode === "shadow" ? qs.today_sales_total || 0 : (d.inv_subscribers || {}).today || 0;
  const box = (cls, num, lbl, ico) => `<div class="small-box ${cls}"><div class="inner"><h3>${num}</h3><p>${lbl}</p></div><div class="icon"><i class="fa ${ico}"></i></div><a class="small-box-footer">تفاصيل أخرى <i class="fa fa-arrow-circle-left"></i></a></div>`;
  const TYPES = [["same", "تفعيل بنفس الباقة"], ["time", "تفعيل بالوقت"], ["card", "تفعيل بالكارت"], ["cash", "تفعيل بمبلغ نقدي"], ["quota", "إضافة كوتة من رصيد المدير"],
    ["borrow", "إضافة سلفة"], ["money", "إضافة مبلغ نقدي"], ["trx", "التفعيل برقم المعاملة"]];
  C(`<div class="row3">${box("bg-green", typeof d.balance === "number" ? fmt(d.balance) : esc(d.balance), "الرصيد", "fa-money")}${box("bg-red", "<span id=\"cDebt\">0.00</span> EGP", "الديون", "fa-minus-circle")}
      ${box("bg-blue-steel", fmt(today) + " EGP", "محصلة الفواتير اليومية", "fa-calendar")}</div>
    <div class="adv-card"><div class="adv-head"><span><i class="fa fa-bolt"></i> لوحة الشحن</span></div><div class="adv-body">
      <div class="g12">
        <div class="fg s4"><label>اسم المستخدم</label><div class="adv-inline"><span class="adv-cal"><i class="fa fa-search"></i></span><input id="cUser" placeholder="بحث" autocomplete="off" list="cList"></div><datalist id="cList"></datalist></div>
        <div class="fg s4"><label>الاسم</label><input id="cName" readonly></div>
        <div class="fg s4"><label>الباقة</label><input id="cProf" readonly></div>
        <div class="fg s4"><label>طريقة التفعيل</label><select id="cType">${TYPES.map(([k, t]) => `<option value="${k}">${t}</option>`).join("")}</select></div>
        <div class="fg s4"><label>عدد مرات التفعيل</label><input id="cCount" type="number" min="1" value="1" placeholder="عدد مرات التفعيل"></div>
        <div class="fg s4"><label>&nbsp;</label><div class="adv-inline" style="gap:6px">
          <button class="btn-red" id="cGo">تفعيل</button><button class="btn-lblue" id="cProfBtn">تغيير العرض</button>
          <button class="btn-lblue" id="cInv">طباعة آخر فاتورة</button><button class="b-green" id="cRec">طباعة آخر إيصال</button></div></div>
      </div>
      <div id="cInfo" style="margin-top:14px"></div></div></div>`);
  let sub = null, t;
  const pick = async name => {
    const r = await api("subscribers?size=1&q=" + encodeURIComponent(name));
    sub = r.rows.find(x => x.username === name) || null;
    $("#cDebt").textContent = sub ? fmt((await api(`subscribers/${sub.id}/quote`)).debt || 0) : "0.00";
    $("#cName").value = sub ? `${sub.firstname || ""} ${sub.lastname || ""}` : "";
    $("#cProf").value = sub ? sub.profile || "" : "";
    $("#cInfo").innerHTML = sub ? `<div class="kv"><div>ينتهي</div><div>${esc(advDate(sub.expiration) || "---")}</div><div>الحالة</div><div>${statusTag(sub)} ${onlineTag(sub)}</div>
      <div>الكوتة المتبقية</div><div>${+sub.profile_quota ? bytesTxt(sub.quota_mb - sub.used_mb) : "غير محدود"}</div><div>الموبايل</div><div>${esc(sub.mobile)}</div></div>` : "";
  };
  $("#cUser").oninput = e => {
    clearTimeout(t); const v = e.target.value.trim(); if (v.length < 2) return;
    t = setTimeout(async () => {
      const r = await api("subscribers?size=15&q=" + encodeURIComponent(v));
      $("#cList").innerHTML = r.rows.map(x => `<option value="${esc(x.username)}">${esc((x.firstname || "") + " — " + (x.mobile || ""))}</option>`).join("");
      if (r.rows.some(x => x.username === v)) pick(v);
    }, 300);
  };
  $("#cUser").onchange = e => pick(e.target.value.trim());
  const need = () => { if (!sub) { toast("اختار المشترك الأول", true); return false; } return true; };
  $("#cGo").onclick = async () => {
    if (!need()) return;
    const type = $("#cType").value, n = Math.max(1, +$("#cCount").value || 1);
    if (type === "time") { const days = prompt("عدد الأيام:", "1"); if (days) await act(() => api(`subscribers/${sub.id}/extend`, { method: "POST", body: { Days: +days, Reason: "لوحة الشحن" } }), "تم"); return pick(sub.username); }
    const viaModal = { cash: "RECH_CASH", quota: "ADD_NEW_QUOTA", money: "ADD_CASH", trx: "pay_with_wallet" }[type];
    if (viaModal) return USER_OPS[viaModal](sub.id, () => pick(sub.username));
    if (type !== "same" && type !== "borrow") return toast("طريقة التفعيل دي هتتعمل في المرحلة الجاية", true);
    if (!confirm(`تفعيل ${sub.username} ${n} مرة${type === "borrow" ? " (سلفة غير مدفوعة)" : ""}؟`)) return;
    for (let i = 0; i < n; i++) await act(() => api(`subscribers/${sub.id}/renew`, { method: "POST", body: { ProfileID: 0, Discount: 0, Paid: type !== "borrow", Notes: type === "borrow" ? "سلفة" : "لوحة الشحن" } }));
    toast("تم التفعيل"); pick(sub.username);
  };
  $("#cProfBtn").onclick = () => { if (need()) location.hash = "user?id=" + sub.id; };
  $("#cInv").onclick = $("#cRec").onclick = () => { if (need()) printLastInvoice(sub.id); };
};
