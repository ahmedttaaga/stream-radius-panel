// ADV's activation modals (#credit_modal, #add_time_modal, ...) and the right-click menu on the subscriber list, same items and wording.
"use strict";
const UNITS = [["1", "الأيام"], ["2", "ساعات"], ["3", "دقائق"]];
// a modal in ADV's layout: h3 title, مدفوع/غير مدفوع switch at the left of the header, form-horizontal rows (label col-5 / value col-6), footer إلغاء + تفعيل
function advModal({ title, paidSwitch, rows, footer = "تفعيل", onSubmit, wide }) {
  const m = modal(`<div class="adv-modal ${wide ? "wide" : ""}"><div class="adv-mh"><h3>${title}</h3>${paidSwitch ? `<label class="paid-sw"><input type="checkbox" id="mPaid" checked><span class="on">مدفوع</span><span class="off">غير مدفوع</span></label>` : ""}</div>
    <form id="advF" class="adv-mb">${rows.map(([l, v, extra]) => `<div class="fh ${extra || ""}"><label class="fl">${l}</label><div class="fv">${v}</div></div>`).join("")}</form>
    <div class="adv-mf"><button type="button" class="btn-cancel" data-close>إلغاء</button><button type="button" class="btn-primary w100" id="mGo">${footer}</button></div></div>`);
  const go = async () => {
    const f = formData($("#advF", m)), paid = paidSwitch ? $("#mPaid", m).checked : true;
    $("#mGo", m).disabled = true;
    try { await onSubmit(f, paid); closeModal(); } catch (e) { toast(e.message, true); } finally { if ($("#mGo", m)) $("#mGo", m).disabled = false; }
  };
  $("#mGo", m).onclick = go;
  $("#advF", m).onsubmit = e => { e.preventDefault(); go(); };
  if (paidSwitch) $("#mPaid", m).onchange = e => m.querySelectorAll(".paid-only").forEach(x => x.classList.toggle("hidden", !e.target.checked));
  return m;
}
const lbl = v => `<span class="cl">${esc(v ?? "")}</span>`;
const qRows = (q, extra = []) => [["اسم المستخدم", `<b dir="auto">${esc(q.username)}</b>`], ...extra,
  ["وقت الانتهاء", lbl(advDate(q.expiration) || "---")], ["الباقة", lbl(q.profile || "---")], ["السعر", lbl(Number(q.price || 0).toFixed(2))],
  ["الكوتة (Up/Down)", lbl(+q.pquota ? bytesTxt(q.quota_mb - q.used_mb) : "غير محدود")], ["الكوتة الكلية (Up/Down)", lbl(+q.pquota ? gbTxt(q.pquota) : "غير محدود")],
  ["الرصيد المتاح", lbl(typeof q.manager_balance === "number" ? fmt(q.manager_balance) : q.manager_balance)], ["الديون", lbl(fmt(q.debt))],
  ...(+q.advance ? [["قيمة السلفة", `<span class="cl danger">${fmt(q.advance)}</span>`]] : [])];
const partialRow = (name) => ["المبلغ المدفوع الجزئي", `<input name="${name}" type="number" min="0" step="any" value="0" autocomplete="off"><span class="help">المبلغ الذي تم سداده من إجمالي الفاتورة</span>`, "paid-only"];
const notesRow = (name = "notes") => ["ملاحظات", `<input name="${name}" type="text" autocomplete="off">`];
const timeRow = (name, sel) => ["", `<div class="adv-inline"><input name="${name}" type="number" min="1" value="1"><select name="${sel}">${UNITS.map(([v, t]) => `<option value="${v}">${t}</option>`).join("")}</select></div>`];

// every ADV action, keyed like ADV's buttons / context menu
const USER_OPS = {
  async CREDITS(id, done) { // تفعيل(الرصيد) — #credit_modal
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "تفعيل مشترك", paidSwitch: true, rows: [...qRows(q, [["<span class='red'>عدد مرات التفعيل</span>", '<input name="count" type="text" value="1">']]), partialRow("partial"), notesRow("comment")],
      onSubmit: async (f, paid) => { const r = await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "activate", Count: +f.count || 1, Paid: paid, Partial: +f.partial || 0, Notes: f.comment } }); toast("تم التفعيل لحد " + advDate(r.expiration)); done && done(); } });
  },
  async REF_CARD(id, done) { // شحن بالكارت — #refill_modal
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "شحن بالكارت", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["وقت الانتهاء", lbl(advDate(q.expiration))], ["الباقة", lbl(q.profile)], ["الديون", lbl(fmt(q.debt))],
      ["رقم الكارت", '<input name="card" type="text" dir="ltr" autocomplete="off">'], notesRow("comment")],
      onSubmit: async () => { throw new Error("شحن الكروت هيتفعّل لما كروت الشحن تتنقل من ADV"); } });
  },
  async TRIAL(id) { const q = await api(`subscribers/${id}/quote`); advModal({ title: "تفعيل مشترك", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["وقت الانتهاء", lbl(advDate(q.expiration))], ["الباقة", lbl(q.profile)], ["الرصيد التجريبي المطلوب", lbl("--")], ["الرصيد التجريبي المتاح", lbl("0")], notesRow("comment")], onSubmit: async () => { throw new Error("الرصيد التجريبي هيتعمل في المرحلة الجاية"); } }); },
  async ADD_TIME(id, done) { // إضافة وقت — #add_time_modal
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "إضافة وقت", paidSwitch: true, rows: [timeRow("add_time", "sel_time"), ...qRows(q), partialRow("partial"), notesRow("comment")],
      onSubmit: async (f, paid) => { const r = await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "add_time", Value: +f.add_time, Unit: +f.sel_time, Paid: paid, Partial: +f.partial || 0, Notes: f.comment } }); toast("اتضاف الوقت لحد " + advDate(r.expiration)); done && done(); } });
  },
  async ADD_ADVANCE(id, done) { // إضافة سلفة — #add_ch_modal
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "إضافة سلفة", rows: [timeRow("add_ch", "sel_ch"), ...qRows(q), notesRow("comment")],
      onSubmit: async f => { const r = await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "add_ch", Value: +f.add_ch, Unit: +f.sel_ch, Notes: f.comment } }); toast("اتسجلت السلفة لحد " + advDate(r.expiration)); done && done(); } });
  },
  async SELF_BORROW_BY_ADMIN(id, done) {
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "سلفة من حساب المشترك", rows: [timeRow("add_ch", "sel_ch"), ...qRows(q), ["رصيد المشترك", lbl(fmt(q.money))], notesRow("comment")],
      onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "self_borrow", Value: +f.add_ch, Unit: +f.sel_ch, Notes: f.comment } }); toast("تم"); done && done(); } });
  },
  async ADD_NEW_QUOTA(id, done) { // إضافة كوتة من رصيد المدير
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "إضافة كوتة", paidSwitch: true, rows: [["الكوتة", `<div class="adv-inline"><input name="qv" type="number" min="0" step="any" value="1"><select name="qu"><option value="1024">Gbps</option><option value="1">Mbps</option><option value="1048576">Tbps</option></select></div>`], ...qRows(q), notesRow("comment")],
      onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "add_quota", Amount: (+f.qv || 0) * (+f.qu), Notes: f.comment } }); toast("اتضافت الكوتة"); done && done(); } });
  },
  async ADD_NEW_QUOTA_U(id, done) { return USER_OPS.ADD_NEW_QUOTA(id, done); },
  async RECH_CASH(id, done) { // شحن بالرصيد النقدي — #charge_credit_modal
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "شحن بالرصيد النقدي", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["المبلغ", `<input name="amount" type="text" value="${Number(q.price || 0).toFixed(2)}" readonly>`], ["وقت الانتهاء", lbl(advDate(q.expiration))], ["الباقة", lbl(q.profile)],
      ["رصيد المشترك", lbl(fmt(q.money))], ["الديون", lbl(fmt(q.debt))], notesRow("comment")],
      onSubmit: async f => { const r = await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "charge_credit", Notes: f.comment } }); toast("تم التفعيل لحد " + advDate(r.expiration)); done && done(); } });
  },
  async ADD_CASH(id, done) { // إضافة مبلغ نقدي — #charge_money_modal
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "إضافة مبلغ نقدي", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["المبلغ", '<input name="amount" type="number" min="0" step="any" autocomplete="off">'], ["الرصيد المتاح", lbl(typeof q.manager_balance === "number" ? fmt(q.manager_balance) : q.manager_balance)], ["رصيد المشترك", lbl(fmt(q.money))], notesRow("comment")],
      onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "charge_money", Amount: +f.amount, Notes: f.comment } }); toast("تم"); done && done(); } });
  },
  async WITHDRAW_CASH(id, done) {
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "سحب مبلغ نقدي", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["المبلغ", '<input name="amount" type="number" min="0" step="any" autocomplete="off">'], ["رصيد المشترك", lbl(fmt(q.money))], notesRow("comment")],
      onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "withdraw_money", Amount: +f.amount, Notes: f.comment } }); toast("تم"); done && done(); } });
  },
  async pay_with_wallet(id, done) { // التفعيل برقم المعاملة: the employee types what the customer's screenshot shows and picks the payment
    const q = await api(`subscribers/${id}/quote`);
    let pick = "";
    const m = advModal({ title: "التفعيل برقم المعاملة", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`],
      ["ابحث عن التحويل", '<input name="trx" type="text" autocomplete="off" placeholder="رقم المعاملة / رقم العملية / رقم المحفظة / عنوان إنستاباي / الاسم">'],
      ["التحويل", '<div id="trxInfo" class="muted">اكتب اللي في صورة التحويل اللي العميل بعتها</div>'], ["رصيد المشترك", lbl(fmt(q.money))], ["الباقة", lbl(q.profile)], ["السعر", lbl(Number(q.price || 0).toFixed(2))],
      ["", '<label><input type="checkbox" name="renew" checked> جدد الباقة لو الرصيد يكفي</label>'], notesRow("comment")],
      onSubmit: async f => { if (!pick) throw new Error("اختار التحويل الأول");
        const r = await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "trx", Target: pick, Count: f.renew ? 1 : 0, Notes: f.comment } });
        toast(`اتضاف ${Number(r.amount).toFixed(2)} للرصيد` + (r.expiration ? " — واتجدد لحد " + advDate(r.expiration) : r.note ? " — " + r.note : "")); done && done(); } });
    const inp = m.querySelector('input[name="trx"]'), info = m.querySelector("#trxInfo");
    let timer = 0, seq = 0;
    const show = rows => {
      if (!rows.length) { info.textContent = "مفيش تحويل جديد بالبيانات دي — اتأكد من اللي في الصورة"; pick = ""; return; }
      info.innerHTML = rows.map((t, i) => { const ok = t.status === "new";
        return `<label style="display:block;padding:6px 0;border-bottom:1px solid #eee;${ok ? "" : "opacity:.6"}"><input type="radio" name="trxPick" value="${esc(String(t.code))}" ${ok ? "" : "disabled"} ${ok && i === 0 ? "checked" : ""}>
          <b>${Number(t.amount).toFixed(2)} EGP</b> — ${esc(t.sender_name || "")} <span dir="ltr">${esc(t.wallet || "")}</span><br><small>${esc(t.source_text || "")} · ${esc(String(t.tx_time || "").replace("T", " ").slice(0, 16))} · رقم المعاملة <b dir="ltr">${esc(String(t.code))}</b>${ok ? "" : ` · <b style="color:#e11d48">${t.status === "used" ? "مستخدمة لـ " + esc(t.used_for || "") : "ملغية"}</b>`}</small></label>`; }).join("");
      const first = rows.find(t => t.status === "new"); pick = first ? String(first.code) : "";
      info.querySelectorAll('input[name="trxPick"]').forEach(x => x.onchange = () => { pick = x.value; });
    };
    inp.oninput = () => { clearTimeout(timer); const v = inp.value.trim(); pick = "";
      if (v.length < 4) { info.textContent = "اكتب 4 حروف أو أرقام على الأقل"; return; }
      info.textContent = "بيدوّر...";
      timer = setTimeout(async () => { const my = ++seq; try { const rows = await api("cash/find?q=" + encodeURIComponent(v)); if (my === seq) show(rows); } catch (e) { if (my === seq) info.textContent = e.message; } }, 500); };
  },
  async trans_money(id, done) {
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "تحويل رصيد من مشترك لآخر", rows: [["من", `<b>${esc(q.username)}</b>`], ["رصيد المشترك", lbl(fmt(q.money))], ["إلى (اسم المستخدم)", '<input name="to" type="text" dir="ltr" autocomplete="off">'], ["المبلغ", '<input name="amount" type="number" min="0" step="any">'], notesRow("comment")],
      onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "trans_money", Target: f.to, Amount: +f.amount, Notes: f.comment } }); toast("تم التحويل"); done && done(); } });
  },
  async PART_PAY(id, done) {
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "دفع جزئي", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["الديون", lbl(fmt(q.debt))], ["المبلغ المدفوع", '<input name="amount" type="number" min="0" step="any">']],
      footer: "دفع", onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "part_pay", Amount: +f.amount } }); toast("تم"); done && done(); } });
  },
  async change_profile(id, done) {
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "تغيير العرض", rows: [["اسم المستخدم", `<b>${esc(q.username)}</b>`], ["العرض الحالي", lbl(q.profile)], ["العرض الجديد", `<select name="p">${options(PROFILES, 0)}</select>`]],
      footer: "تغيير", onSubmit: async f => { if (!+f.p) throw new Error("اختار العرض"); await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "change_profile", ProfileID: +f.p } }); toast("اتغير العرض"); done && done(); } });
  },
  async rename_user(id, done) {
    const q = await api(`subscribers/${id}/quote`);
    advModal({ title: "إعادة تسمية", rows: [["الاسم الحالي", `<b>${esc(q.username)}</b>`], ["الاسم الجديد", '<input name="nu" type="text" dir="ltr" autocomplete="off">']],
      footer: "حفظ", onSubmit: async f => { await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "rename", Target: f.nu } }); toast("تم"); done && done(); } });
  },
  async delete_user(id, done) { if (confirm("هل أنت متأكد من حذف المشترك؟")) { await act(() => api(`subscribers/${id}/op`, { method: "POST", body: { Action: "delete" } }), "اتحذف"); done && done(); } },
  async disconnect_user(id, done) { if (confirm("فصل اتصال المشترك؟")) { await act(() => api(`subscribers/${id}/op`, { method: "POST", body: { Action: "disconnect" } }), "اتبعت أمر الفصل"); done && done(); } },
  async send_msg(id) {
    advModal({ title: "رسالة جديدة", rows: [["نوع الرسالة:", `<select name="kind"><option>WhatsApp</option><option>إرسال رمز تأكيد لبوت الواتساب</option><option>📱 إشعار التطبيق</option><option>📱 إشعار التطبيق (popup)</option></select>`], ["نص الرسالة:", '<textarea name="text" rows="4"></textarea>']],
      footer: "إرسال", onSubmit: async f => { const r = await api(`subscribers/${id}/op`, { method: "POST", body: { Action: "send_msg", Target: f.kind, Text: f.text } }); toast(r.note || "تم"); } });
  },
  async get_trafic(id) {
    const [d, old, du] = await Promise.all([api("subscribers/" + id), api(`subscribers/${id}/adv-sessions`).catch(() => []), api(`reports/daily-usage?id=${id}&from=${new Date(Date.now() - 30 * 864e5).toISOString().slice(0, 10)}`).catch(() => ({ data: [] }))]);
    modal(`<div class="adv-modal wide"><div class="adv-mh"><h3>الرسم البياني لحركة البيانات</h3></div><div class="adv-mb"><div class="fh"><label class="fl">اسم المستخدم</label><div class="fv"><b>${esc(d.subscriber.username)}</b></div></div>
      <div class="tablewrap">${table(d.sessions, [["acctstarttime", "بداية"], ["acctstoptime", "نهاية"], ["acctoutputoctets", "تحميل", v => bts(v)], ["acctinputoctets", "رفع", v => bts(v)], ["nasipaddress", "NAS"]])}</div>
      ${du.data.length ? `<h4>الاستهلاك اليومي (من 12 بالليل لـ 12 بالليل) — آخر 30 يوم</h4><div class="tablewrap">${table(du.data, [["day", "اليوم"], ["download", "تحميل", v => bts(v)], ["upload", "رفع", v => bts(v)], ["total", "الإجمالي", v => bts(v)]])}</div>` : ""}
      ${old.length ? `<h4>جلسات قديمة من أرشيف ADV (${fmt(old.length)})</h4><div class="tablewrap">${table(old, [["start", "بداية"], ["stop", "نهاية"], ["bytes_out", "تحميل", v => bts(v)], ["bytes_in", "رفع", v => bts(v)], ["nas", "NAS"], ["ip", "IP"], ["mac", "MAC"]])}</div>` : ""}
      </div><div class="adv-mf"><button class="btn-cancel" data-close>إغلاق</button></div></div>`);
  },
  info(id) { location.hash = "user?id=" + id; },
  print_receipt(id) { printLastInvoice(id); },
};
// ADV's right-click menu on a subscriber row (same order/wording)
const CTX_ITEMS = [["send_msg", "إرسال رسالة"], ["CREDITS", "تفعيل(الرصيد)"], ["REF_CARD", "شحن بالكارت"], ["ADD_TIME", "إضافة وقت"], ["ADD_ADVANCE", "إضافة سلفة"],
  ["SELF_BORROW_BY_ADMIN", "سلفة من حساب المشترك"], ["ADD_NEW_QUOTA", "إضافة كوتة من رصيد المدير"], ["ADD_NEW_QUOTA_U", "إضافة كوتة من رصيد المشترك"], ["RECH_CASH", "شحن بالرصيد النقدي"],
  ["pay_with_wallet", "التفعيل برقم المعاملة"], ["trans_money", "تحويل رصيد من مشترك لآخر"], ["ADD_CASH", "إضافة مبلغ نقدي"], ["get_trafic", "مشاهدة الترافيك"], ["PART_PAY", "دفع جزئي"],
  ["DISCONNECT", "فصل الإتصال"], ["sep"], ["copytext", "نسخ القيمة"], ["info", "معلومات"], ["delete", "حذف"]];
const CTX_MAP = { DISCONNECT: "disconnect_user", delete: "delete_user" };
function showContextMenu(e, id, username, done) {
  e.preventDefault();
  document.querySelectorAll(".ctx-menu").forEach(x => x.remove());
  const m = document.createElement("div"); m.className = "ctx-menu";
  m.innerHTML = `<div class="ctx-title">${esc(username)}</div>` + CTX_ITEMS.map(([k, t]) => k === "sep" ? '<div class="ctx-sep"></div>' : `<a data-k="${k}" class="${k === "delete" ? "ctx-del" : ""}">${t}</a>`).join("");
  document.body.appendChild(m);
  const x = Math.min(e.clientX, innerWidth - m.offsetWidth - 8), y = Math.min(e.clientY, innerHeight - m.offsetHeight - 8);
  m.style.left = x + "px"; m.style.top = y + "px";
  m.querySelectorAll("a").forEach(a => a.onclick = () => {
    m.remove();
    const k = CTX_MAP[a.dataset.k] || a.dataset.k;
    if (k === "copytext") return navigator.clipboard.writeText(username).then(() => toast("اتنسخ"));
    if (USER_OPS[k]) USER_OPS[k](id, done); else toast("الخاصية دي هتتعمل في المرحلة الجاية", true);
  });
  setTimeout(() => document.addEventListener("click", () => m.remove(), { once: true }), 0);
}
