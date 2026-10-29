// Our own WhatsApp gateway (link by QR) + SMS through the GoIP. Shown on top of ADV's Whatsapp / SMS settings tabs, plus a "سجل الرسائل" page.
"use strict";
async function waPanel(el) {
  const s = await api("wa/status").catch(e => ({ state: "down", error: e.message }));
  const st = { connected: ["متصل", "green"], qr: ["مستني مسح QR", "orange"], reconnecting: ["بيعيد الاتصال", "orange"], starting: ["بيبدأ", "orange"], logged_out: ["مش مربوط", "red"], down: ["البوابة واقفة", "red"] }[s.state] || [s.state, "gray"];
  el.innerHTML = `<div class="wa-box"><div class="wa-info"><h4><i class="fa fa-whatsapp"></i> بوابة الواتساب الخاصة بالشركة</h4>
      <div class="kv"><div>رقم الشركة</div><div dir="ltr">${esc(s.number || "")}</div><div>الحالة</div><div><span class="tag ${st[1]}">${st[0]}</span>${s.me ? ` <span dir="ltr">(+${esc(s.me)})</span>` : ""}</div>
        <div>في الطابور</div><div>${s.queued ?? 0}</div><div>اتبعت النهارده</div><div>${s.sent_today ?? 0}</div><div>فشل النهارده</div><div>${s.failed_today ?? 0}</div></div>
      <p class="muted">⚠️ البوابة دي بتستخدم واتساب ويب (طريقة غير رسمية). عشان الرقم ما يتقفلش: رسالة كل ${esc(document.querySelector("[name=wa_gap]")?.value || 8)} ثواني، ورسايل العمليات بس.</p>
      <div class="adv-actions"><button type="button" class="b-light" id="waRef"><i class="fa fa-refresh"></i> تحديث</button>
        ${s.state === "connected" ? '<button type="button" class="btn-red" id="waOut">فك الربط</button>' : ""}</div>
      <div class="g12" style="margin-top:10px"><div class="fg s6"><label>رقم تجربة</label><input id="waTo" dir="ltr" placeholder="01xxxxxxxxx"></div><div class="fg s6"><label>نص التجربة</label><input id="waTxt" value="رسالة تجربة من نظام ${esc(ME.company || "Stream-Net")}"></div></div>
      <button type="button" class="b-green" id="waTest" ${s.state === "connected" ? "" : "disabled"}>إرسال تجربة</button></div>
    <div class="wa-qr">${s.qr ? `<img src="${s.qr}" alt="QR"><p>افتح واتساب على موبايل الشركة ← الأجهزة المرتبطة ← ربط جهاز، وامسح الكود</p>` : s.state === "connected" ? '<i class="fa fa-check-circle wa-ok"></i><p>مربوط</p>' : `<p class="muted">${esc(s.error || "مستني الكود...")}</p>`}</div></div>`;
  $("#waRef").onclick = () => waPanel(el);
  const out = $("#waOut"); if (out) out.onclick = async () => { if (confirm("فك ربط رقم الواتساب؟")) { await act(() => api("wa/logout", { method: "POST" }), "اتفك الربط"); setTimeout(() => waPanel(el), 2500); } };
  $("#waTest").onclick = async () => { await act(() => api("messages/test", { method: "POST", body: { Channel: "whatsapp", To: $("#waTo").value, Text: $("#waTxt").value } }), "اتبعتت"); waPanel(el); };
  if (s.state === "qr" || s.state === "starting" || s.state === "reconnecting") { clearTimeout(window.__waT); window.__waT = setTimeout(() => { if (document.body.contains(el)) waPanel(el); }, 6000); }
}
function smsPanel(el, S) {
  el.innerHTML = `<div class="wa-box"><div class="wa-info" style="flex:1"><h4><i class="fa fa-commenting"></i> إرسال SMS عن طريق الـ GoIP</h4>
    <div class="g12"><div class="fg s3"><label>تفعيل</label><label class="onoff"><input type="checkbox" name="goip_enabled" ${S.goip_enabled === "1" ? "checked" : ""}><b data-on="ON" data-off="OFF"></b></label></div>
      <div class="fg s3"><label>IP الـ GoIP</label><input name="goip_ip" dir="ltr" value="${esc(S.goip_ip || "192.168.1.210")}"></div>
      <div class="fg s2"><label>الخط (Line)</label><select name="goip_line">${["1", "2", "3", "4"].map(l => `<option ${S.goip_line === l ? "selected" : ""}>${l}</option>`).join("")}</select></div>
      <div class="fg s2"><label>يوزر الـ GoIP</label><input name="goip_user" dir="ltr" value="${esc(S.goip_user || "")}" autocomplete="off"></div>
      <div class="fg s2"><label>باسورد الـ GoIP</label><input name="goip_pass" type="password" value="${esc(S.goip_pass || "")}" autocomplete="new-password"></div></div>
    <p class="muted">الـ GoIP شغّال مع السنترال. قبل كل رسالة النظام بيقرا حالة الخط، ومش بيبعت غير لما الخط يكون فاضي من المكالمات (بيستنى لحد 3 دقايق). بين كل SMS والتانية 10 ثواني على الأقل، ولو حصل أي خطأ الـ SMS بيتقفل لوحده لحد ما تفتحه.
      ${S.goip_last_error ? `<br><b class="red">آخر خطأ: ${esc(S.goip_last_error)}</b>` : ""}</p>
    <div class="g12"><div class="fg s6"><label>رقم تجربة</label><input id="smTo" dir="ltr" placeholder="01xxxxxxxxx"></div><div class="fg s6"><label>نص التجربة</label><input id="smTxt" value="SMS test"></div></div>
    <button type="button" class="b-green" id="smTest">إرسال SMS تجربة</button></div></div>`;
  // save the GoIP fields first (so the test uses what is on screen), then send
  $("#smTest").onclick = async () => {
    const g = n => el.querySelector(`[name=${n}]`);
    await act(() => api("settings", { method: "PUT", body: { goip_enabled: g("goip_enabled").checked ? "1" : "0", goip_ip: g("goip_ip").value.trim(), goip_line: g("goip_line").value,
      goip_user: g("goip_user").value.trim(), goip_pass: g("goip_pass").value } }));
    await act(() => api("messages/test", { method: "POST", body: { Channel: "sms", To: $("#smTo").value, Text: $("#smTxt").value } }), "اتبعتت");
  };
}
PAGES.messages = async () => {
  const rows = await api("messages");
  const stTag = s => ({ sent: '<span class="mini green">اتبعتت</span>', failed: '<span class="mini red">فشلت</span>', queued: '<span class="mini gray">في الطابور</span>' })[s] || s;
  C(`<div class="adv-card"><div class="adv-head"><span><i class="fa fa-envelope"></i> سجل الرسائل</span><button class="icon-btn" id="reload"><i class="fa fa-refresh"></i></button></div><div class="adv-body">
    <div class="tablewrap">${table(rows, [["id", "#"], ["created", "الوقت", v => advDate(v)], ["kind", "النوع", v => v === "sms" ? "SMS" : "WhatsApp"], ["username", "المشترك"], ["phone", "الرقم"], ["body", "النص"],
      ["status", "الحالة", v => stTag(v)], ["error", "الخطأ"], ["created_by", "بواسطة"]])}</div></div></div>`);
  $("#reload").onclick = () => PAGES.messages();
};
