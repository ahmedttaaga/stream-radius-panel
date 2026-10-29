// Demo mode: the panel runs with NO server. Every /api/ call is answered here from an invented in-memory dataset, so a
// designer can open index.html straight from disk (file://) or with ?demo and see every page. Nothing here is real:
// names, phones (0100000xxxx), IPs (10.0.0.x / 192.0.2.x), wallets (demoN@instapay) are all made up.
// Turns on when: the page is opened from file://, the URL has ?demo, or /api/me does not answer (no server behind it).
// ?demo&mode=live shows the panel in "Live" mode instead of the default "Shadow" mode.
"use strict";
(() => {
  const realFetch = window.fetch ? window.fetch.bind(window) : null;
  const URLQ = new URLSearchParams(location.search);
  let ON = location.protocol === "file:" || URLQ.has("demo");
  let MODE = URLQ.get("mode") || (() => { try { return sessionStorage.getItem("rm_demo_mode") || "shadow"; } catch (_) { return "shadow"; } })();
  if (MODE !== "live") MODE = "shadow";

  // ---------- helpers (seeded random, so every reload shows the same data)
  let seed = 20261029;
  const rnd = () => { seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
  const ri = (a, b) => a + Math.floor(rnd() * (b - a + 1));
  const pick = a => a[Math.floor(rnd() * a.length)];
  const NOW = Date.now(), DAY = 864e5, HOUR = 36e5, GBB = 1073741824;
  const p2 = n => String(n).padStart(2, "0");
  const dt = ms => { const d = new Date(ms); return `${d.getFullYear()}-${p2(d.getMonth() + 1)}-${p2(d.getDate())} ${p2(d.getHours())}:${p2(d.getMinutes())}:${p2(d.getSeconds())}`; };
  const dOnly = ms => dt(ms).slice(0, 10);
  const iso = ms => dt(ms).replace(" ", "T");
  const unix = ms => Math.floor(ms / 1000);
  const ago = (d, h = 0, m = 0) => NOW - d * DAY - h * HOUR - m * 6e4;
  const phone = n => "0100000" + String(n).padStart(4, "0");
  const mac = n => "02:00:00:" + [n >> 8 & 255, n & 255, (n * 7) & 255].map(x => x.toString(16).padStart(2, "0").toUpperCase()).join(":");
  const parseDT = s => s ? new Date(String(s).replace(" ", "T")).getTime() : 0;
  const clone = v => JSON.parse(JSON.stringify(v));
  const inRange = (ms, from, to) => (!from || ms >= parseDT(from + " 00:00:00")) && (!to || ms <= parseDT(to + " 23:59:59"));

  // ---------- profiles
  const PROFS = [[1, "20M - 200 جيجا", 260, 20, 204800], [2, "30M", 320, 30, 0], [3, "50M", 355, 50, 0], [4, "70M", 400, 70, 0], [5, "100M", 530, 100, 0]]
    .map(([id, name, price, m, q]) => ({ id, adv_id: 100 + id, name, price, down_speed: m * 1024, up_speed: m * 256, rate_limit: `${m / 4}M/${m}M`, days: 30, quota_mb: q,
      pool: "pool-main", simul: 1, users: 0, enabled: 1, f: { exp_unit: "30", exp_unit_val: "2", type_prof: "0", profile_down_qouta: "0", profile_up_qouta: "0",
        pool_name: "pool-main", qouta_expire_service: q ? "106" : "0", daily_expire_service: "0", profile_expire_service: "0" } }));
  PROFS.push({ id: 6, adv_id: 106, name: "END", price: 0, down_speed: 256, up_speed: 256, rate_limit: "256k/256k", days: 0, quota_mb: 0, pool: "pool-end", simul: 1, users: 0, enabled: 0,
    f: { exp_unit: "0", exp_unit_val: "2", type_prof: "1", pool_name: "pool-end" } });
  const prof = id => PROFS.find(p => p.id === +id) || {};

  // ---------- managers (sub-distributors)
  const MGRS = [
    { id: 1, adv_id: 11, username: "nour.center", full_name: "نور الدين عبد الغني", phone: phone(101), balance: 4250, debt: 0, parent: "admin", parent_id: 0, grp: "موزعين", trial: 0, unlimited: 0, enabled: 1 },
    { id: 2, adv_id: 12, username: "hassan.market", full_name: "حسن البدري", phone: phone(102), balance: 1830.5, debt: 320, parent: "admin", parent_id: 0, grp: "موزعين", trial: 50, unlimited: 0, enabled: 1 },
    { id: 3, adv_id: 13, username: "samy.west", full_name: "سامي فكري", phone: phone(103), balance: 0, debt: 0, parent: "nour.center", parent_id: 1, grp: "كافيهات", trial: 0, unlimited: 1, enabled: 1 },
  ];
  const mgr = id => MGRS.find(m => m.id === +id) || null;
  const LEDGER = {};
  MGRS.forEach(m => { let b = m.balance; LEDGER[m.id] = Array.from({ length: 6 }, (_, i) => { const delta = i % 3 === 2 ? -ri(300, 700) : ri(5, 20) * 100; const row = { created: dt(ago(i * 4 + 1, ri(1, 9))), delta, balance_after: b, reason: delta > 0 ? "شحن رصيد" : "تجديد مشتركين", created_by: "admin" }; b -= delta; return row; }); });

  // ---------- NAS (two PPPoE servers)
  const NAS = [{ id: 1, name: "Server-A", ip: "192.0.2.11", type: "mikrotik", description: "سيرفر PPPoE رئيسي (تجربة)" }, { id: 2, name: "Server-B", ip: "192.0.2.12", type: "mikrotik", description: "سيرفر احتياطي (تجربة)" }];

  // ---------- subscribers (~40, invented Egyptian-style names)
  const FN = [["محمد", "mohamed"], ["أحمد", "ahmed"], ["محمود", "mahmoud"], ["مصطفى", "mostafa"], ["عمر", "omar"], ["يوسف", "youssef"], ["خالد", "khaled"], ["إبراهيم", "ibrahim"],
    ["حسن", "hassan"], ["علي", "ali"], ["كريم", "karim"], ["طارق", "tarek"], ["سامح", "sameh"], ["هاني", "hany"], ["وليد", "waleed"], ["شريف", "sherif"], ["ياسر", "yasser"],
    ["رامي", "ramy"], ["فاطمة", "fatma"], ["منى", "mona"], ["هبة", "heba"], ["نورا", "nora"], ["سارة", "sara"], ["دينا", "dina"], ["إيمان", "eman"], ["مروة", "marwa"]];
  const LN = ["عبد الحميد", "السيد", "فؤاد", "الشناوي", "رمضان", "عبد الله", "حسنين", "منصور", "البنا", "زكي", "عطية", "الجندي", "سليمان", "شعبان", "عبد الرحمن", "الشافعي", "نصار", "الفقي"];
  const STREETS = ["شارع المدرسة", "شارع الجامع", "شارع البحر", "عمارة 4 - المساكن", "برج النور", "شارع السوق", "حارة الصيادين", "شارع المحطة"];
  const NOTES = ["", "", "", "بيدفع أول الشهر", "راوتر TP-Link", "عايز يزود السرعة", "", "بيتصل قبل التجديد"];
  // expiry plan per row: days from now (negative = expired), plus a few special cases
  const EXP_PLAN = [22, 15, 9, 27, -4, 18, 2, 11, 25, -12, 6, 30, 0.3, 14, 19, -1, 8, 24, 1.5, 12, 26, -20, 5, 17, 21, 3, 28, -7, 10, 13, 16, 2.5, 23, -2, 20, 7, 29, 4, -30, 18];
  const SUBS = EXP_PLAN.map((days, i) => {
    const [fa, fe] = FN[i % FN.length], ln = LN[(i * 7) % LN.length], p = PROFS[[1, 2, 2, 3, 1, 4, 2, 0, 3, 1][i % 10]], m = MGRS[i % 4 === 3 ? 1 : i % 5 === 4 ? 2 : 0];
    const enabled = i === 9 || i === 27 ? 0 : 1, exp = days ? NOW + days * DAY + ri(1, 600) * 6e4 : 0;
    const quota = p.quota_mb, used = quota ? (i === 7 ? quota : ri(10, 190) * 1024) : ri(20, 400) * 1024;
    return { id: 1001 + i, adv_id: 5001 + i, username: `${fe}${10 + i}`, firstname: fa, lastname: ln, mobile: phone(1000 + i * 37), address: `${pick(STREETS)} - رقم ${ri(1, 60)}`,
      notes: NOTES[i % NOTES.length], profile_id: p.id, profile: p.name, profile_quota: quota, quota_mb: quota, used_mb: used, manager_id: m.id, manager: m.username,
      expiration: exp ? dt(exp) : "", created: dt(ago(ri(40, 700), ri(0, 12))), enabled, online: 0, ip: "", nas: "", nas_name: "", mac: mac(300 + i), framed_ip: i === 5 || i === 17 ? `10.0.0.${200 + i}` : "",
      money: i % 6 === 0 ? ri(1, 30) * 10 : 0, paid_total: 0, unpaid_total: i % 11 === 3 ? p.price : 0, discount: i % 13 === 6 ? 10 : 0, borrowed: i % 11 === 3 ? 1 : 0,
      simul: 1, auto_renew: i % 9 === 0 ? 1 : 0, source: i % 4 === 1 ? "local" : "adv", extra: JSON.stringify({ email: "", country: "EG", instapay: i % 5 === 0 ? `demo${i}@instapay` : "", num_doc: String(2000 + i) }) };
  });
  // ~15 online, all active
  SUBS.filter(s => +s.enabled && parseDT(s.expiration) > NOW).slice(0, 15).forEach((s, k) => {
    const n = NAS[k % 3 === 2 ? 1 : 0]; s.online = 1; s.ip = `10.0.0.${20 + k * 3}`; s.nas = n.ip; s.nas_name = n.name;
    s._since = ago(0, ri(0, 30), ri(1, 59)); s._in = ri(1, 30) * GBB / 10; s._out = ri(5, 120) * GBB / 4;
  });
  const sub = id => SUBS.find(s => s.id === +id);
  const subByName = u => SUBS.find(s => s.username === u);

  // ---------- invoices (last 30 days)
  const INV = [];
  let invId = 1;
  SUBS.forEach((s, i) => {
    const p = prof(s.profile_id), at = Math.min(NOW - HOUR, Math.max(ago(29), parseDT(s.expiration) - 30 * DAY));
    if (!s.expiration) return;
    const paid = s.borrowed ? 0 : 1;
    INV.push({ id: invId++, sub_id: s.id, created: dt(at), kind: "subscriber", username: s.username, manager: s.manager, profile: p.name, operation: i % 4 === 0 ? "activate" : "renew",
      amount: p.price, discount: s.discount, paid, created_by: i % 3 ? s.manager : "admin", period_from: dt(at), period_to: s.expiration, notes: paid ? "" : "سلفة" });
  });
  MGRS.forEach(m => [3, 11, 19, 26].forEach(d => INV.push({ id: invId++, created: dt(ago(d, ri(1, 8))), kind: "manager", username: "", manager: m.username, profile: "", operation: "deposit",
    amount: ri(5, 20) * 100, discount: 0, paid: d === 26 && m.id === 2 ? 0 : 1, created_by: "admin", period_from: "", period_to: "", notes: "شحن رصيد" })));
  [1, 4, 8, 13, 17, 22, 27].forEach(d => INV.push({ id: invId++, created: dt(ago(d, ri(1, 8))), kind: "card", username: "", manager: pick(MGRS).username, profile: "كارت 2 ساعة", operation: "card",
    amount: ri(4, 12) * 50, discount: 0, paid: 1, created_by: "admin", period_from: "", period_to: "", notes: "حزمة كروت" }));
  INV.sort((a, b) => parseDT(b.created) - parseDT(a.created));
  SUBS.forEach(s => { s.paid_total = INV.filter(x => x.sub_id === s.id && +x.paid).reduce((a, x) => a + x.amount - x.discount, 0); });

  // ---------- cash transactions (Vodafone Cash + InstaPay)
  const SENDERS = ["محمد ع. السيد", "أحمد ف. منصور", "منى ع. زكي", "خالد م. البنا", "هبة س. رمضان", "يوسف ح. نصار", "سارة م. الجندي", "طارق ع. عطية", "دينا ح. الفقي", "وليد س. شعبان"];
  const CASH = Array.from({ length: 25 }, (_, i) => {
    const src = i % 9 === 8 ? "manual" : i % 3 === 0 ? "vfcash" : i % 3 === 1 ? "instapay" : "ipnotif";
    const at = i < 3 ? ago(0, 0, 10 + i * 25) : ago(Math.floor(i * 1.1), ri(0, 10), ri(0, 59)), amount = i % 4 === 3 ? ri(1, 9) * 50 : pick(PROFS.slice(0, 5)).price;
    const status = i === 6 ? "void" : i === 11 ? "dup" : i % 3 === 2 || i > 14 ? "used" : "new";
    const s = SUBS[(i * 5) % SUBS.length];
    return { id: 501 + i, code: String(src === "vfcash" ? 7700000000 + i * 1379 : 8800000000 + i * 2741), source: src,
      source_text: { vfcash: "فودافون كاش", instapay: "إنستاباي (رسالة البنك)", ipnotif: "إشعار إنستاباي", manual: "يدوي" }[src],
      wallet: src === "vfcash" ? phone(2000 + i * 13) : `demo${i + 1}@instapay`, sender_name: src === "vfcash" ? "" : SENDERS[i % SENDERS.length], amount,
      tx_time: iso(at), received: iso(at + ri(5, 90) * 1000), status, used_for: status === "used" ? s.username : "", used_at: status === "used" ? iso(at + ri(10, 300) * 6e4) : "",
      used_by: status === "used" ? (i % 2 ? "admin" : s.manager) : "", ext_ref: src === "instapay" ? "IPN" + (300000 + i * 17) : "", notes: src === "manual" ? "اتسجلت يدوي (تجربة)" : "",
      dup_of: i === 11 ? 501 + 10 : "", merged_id: i === 4 ? 501 + 11 : "", suggest: status === "new" && i % 2 === 0 ? s.username : "" };
  });
  const DEVICES = [
    { id: 1, name: "موبايل المكتب", enabled: 1, paired: 1, idle_min: 3, last_seen: iso(ago(0, 0, 3)), info: JSON.stringify({ model: "Demo Phone A", version: "1.4", sms: true, notif: true, battery: true, listening: true }), pair_code: "", pair_pin: "" },
    { id: 2, name: "موبايل الفرع", enabled: 1, paired: 1, idle_min: 95, last_seen: iso(ago(0, 1, 35)), info: JSON.stringify({ model: "Demo Phone B", version: "1.3", sms: true, notif: false, battery: true, queue: 2 }), pair_code: "", pair_pin: "" },
    { id: 3, name: "موبايل احتياطي", enabled: 0, paired: 0, idle_min: null, last_seen: "", info: "{}", pair_code: "", pair_pin: "" },
  ];
  const CASH_UNKNOWN = [
    { at: iso(ago(0, 2)), device: "موبايل المكتب", sender: "VF-Cash", body: "رسالة تجربة: تم استلام مبلغ بشكل غير معتاد من رقم 01000009999 — الصيغة دي مش مفهومة." },
    { at: iso(ago(1, 5)), device: "موبايل الفرع", sender: "DemoBank", body: "Demo bank message: account credited (format not recognised)." },
  ];

  // ---------- cards (recharge) + card series ("مشتركين الكارت")
  const CARDS = [];
  [["S-" + dOnly(ago(12)).replace(/-/g, ""), 1, 50, 12], ["S-" + dOnly(ago(3)).replace(/-/g, ""), 2, 100, 3]].forEach(([series, pid, price, d], si) => {
    for (let k = 0; k < 10; k++) CARDS.push({ id: CARDS.length + 1, code: String(4000000000 + si * 99991 + k * 7919), series, kind: "recharge", profile: prof(pid).name, price,
      status: k < (si ? 3 : 7) ? "used" : "new", manager: MGRS[si].username, expires: dt(ago(-60 + d)), created: dt(ago(d, 2)) });
  });
  const CARD_SERIES = [
    { name_ser: "كافيه النخيل - 2 ساعة", at_the_first_login: 1, profile: 102, price: 10, simul: 1, created_on: unix(ago(9)), qty: 100, usage_cards: 64, val_time_exp: 2, val_date: 2, owner: 13, exp_date: unix(ago(-50)), all_quota: 2048, exp_quota: 1, connect_by_mac: 1, val_serv_name: "hs-cafe", notes: "" },
    { name_ser: "كافيه الشروق - يوم", at_the_first_login: 1, profile: 103, price: 25, simul: 2, created_on: unix(ago(20)), qty: 50, usage_cards: 50, val_time_exp: 1, val_date: 1, owner: 11, exp_date: 0, all_quota: 0, exp_quota: 0, connect_by_mac: 0, val_serv_name: "", notes: "خلصت" },
  ];

  // ---------- cafés (VPN clients) + café card series
  const CAFES = [
    { id: 1, name: "كافيه النخيل", notes: "تجربة", enabled: 1, connected: true, uptime: "3d4h", ip: "10.0.0.194", username: "cafe-nakheel", phone: phone(301), manager: "samy.west", manager_id: 3, online_cards: 4 },
    { id: 2, name: "كافيه الشروق", notes: "", enabled: 1, connected: false, uptime: "", ip: "10.0.0.195", username: "cafe-shorouk", phone: phone(302), manager: "nour.center", manager_id: 1, online_cards: 0 },
  ];
  const CAFE_SERIES = [
    { id: 1, name: "نخيل-2س", notes: "", cafe: "كافيه النخيل", cafe_id: 1, profile: "30M", qty: 100, available: 36, active: 8, ended: 56, online: 4, time_val: 2, time_unit: 2, time_mode: "first_login", quota_mb: 2048, price: 10, cost: 6, mac_max: 1, created: dt(ago(9)), enabled: 1 },
    { id: 2, name: "نخيل-يوم", notes: "عرض الويك إند", cafe: "كافيه النخيل", cafe_id: 1, profile: "50M", qty: 40, available: 25, active: 3, ended: 12, online: 0, time_val: 1, time_unit: 1, time_mode: "creation", quota_mb: 0, price: 25, cost: 15, mac_max: 2, created: dt(ago(4)), enabled: 1 },
    { id: 3, name: "شروق-ساعة", notes: "", cafe: "كافيه الشروق", cafe_id: 2, profile: "20M - 200 جيجا", qty: 60, available: 60, active: 0, ended: 0, online: 0, time_val: 60, time_unit: 3, time_mode: "online", quota_mb: 512, price: 5, cost: 3, mac_max: 0, created: dt(ago(1)), enabled: 0 },
  ];
  const cafeCards = sid => { const s = CAFE_SERIES.find(x => x.id === +sid); if (!s) return []; const st = ["new", "active", "expired", "quota", "disabled"];
    return Array.from({ length: Math.min(20, s.qty) }, (_, k) => { const status = k < s.active ? "active" : k < s.active + Math.min(5, s.ended) ? pick(["expired", "quota"]) : k === 19 ? st[4] : "new";
      return { username: String(70000000 + s.id * 1000 + k), password: k % 3 ? String(70000000 + s.id * 1000 + k) : "", status, first_login: status === "new" ? "" : dt(ago(0, ri(1, 30))),
        expires: status === "new" ? "" : dt(ago(0, -ri(1, 20))), used_mb: status === "new" ? 0 : ri(10, 900), used_min: status === "new" ? 0 : ri(5, 110), mac: status === "new" ? "" : mac(900 + k), online: status === "active" && k < s.online ? 1 : 0 }; }); };

  // ---------- small lists
  const TICKETS = [
    { id: 1, subject: "النت بيفصل بالليل", username: SUBS[2].username, status: "open", created: dt(ago(1, 3)), created_by: "admin", body: "المشترك بيقول النت بيفصل كل يوم الساعة 11 بالليل." },
    { id: 2, subject: "السرعة أقل من الباقة", username: SUBS[8].username, status: "pending", created: dt(ago(3, 6)), created_by: "nour.center", body: "قاس السرعة لقاها نص الباقة." },
    { id: 3, subject: "تغيير الباسورد", username: SUBS[14].username, status: "closed", created: dt(ago(9, 2)), created_by: "admin", body: "عايز يغير باسورد الواي فاي." },
  ];
  const REPLIES = { 1: [{ created_by: "admin", created: dt(ago(1, 2)), body: "اتعمل فحص للخط، مستنيين الفني." }], 2: [], 3: [{ created_by: "admin", created: dt(ago(9)), body: "اتغير واتقفلت الشكوى." }] };
  const MESSAGES = Array.from({ length: 10 }, (_, i) => { const s = SUBS[i * 3]; return { id: 900 + i, created: dt(ago(Math.floor(i / 2), i * 2)), kind: i % 4 === 3 ? "sms" : "whatsapp",
    username: s.username, phone: s.mobile, body: i % 3 ? `أهلا ${s.firstname}، اشتراكك اتجدد لحد ${s.expiration.slice(0, 10)}` : `تذكير: اشتراكك بينتهي قريب`, status: i === 4 ? "failed" : i === 0 ? "queued" : "sent",
    error: i === 4 ? "الرقم مش على واتساب" : "", created_by: i % 2 ? "admin" : "النظام" }; });
  const AUDIT = Array.from({ length: 16 }, (_, i) => { const s = SUBS[(i * 3) % SUBS.length];
    return { ts: dt(ago(Math.floor(i / 3), i)), admin: i % 3 ? "admin" : s.manager, action: ["renew", "edit_subscriber", "manager_deposit", "login", "toggle", "add_time"][i % 6],
      target: i % 6 === 2 ? MGRS[i % 3].username : s.username, details: JSON.stringify({ profile: s.profile, by: "demo" }), ip: `10.0.0.${150 + i % 5}` }; });
  const ADMINS = [{ id: 1, username: "admin", full_name: "مدير التجربة", role: "admin", enabled: 1, manager_id: 0 }, { id: 2, username: "nour.center", full_name: "نور الدين عبد الغني", role: "manager", enabled: 1, manager_id: 1 },
    { id: 3, username: "support", full_name: "خدمة العملاء", role: "viewer", enabled: 1, manager_id: 0 }, { id: 4, username: "old.staff", full_name: "موظف سابق", role: "viewer", enabled: 0, manager_id: 0 }];
  const TRASH = [{ id: 1, name: SUBS[38].username, kind: "مشترك", info: "منتهي من شهر", deleted_by: "admin", deleted_at: dt(ago(2)) }, { id: 2, name: "S-OLD-01", kind: "حزمة كروت", info: "20 كارت", deleted_by: "admin", deleted_at: dt(ago(6)) },
    { id: 3, name: "40M", kind: "عرض", info: "عرض قديم", deleted_by: "admin", deleted_at: dt(ago(11)) }, { id: 4, name: "Server-Old", kind: "NAS", info: "192.0.2.99", deleted_by: "admin", deleted_at: dt(ago(20)) }];
  const BACKUPS = Array.from({ length: 7 }, (_, i) => ({ name: `backup-${dOnly(ago(i))}-0330.sql.gz`, date: dOnly(ago(i)) + " 03:30:00", size: 4.2e6 + i * 31e3 }));
  let SPEED = { state: "off", mode: "off", until: "", from: "02:00", to: "08:00", type: "multiply", value: 2 };
  const SETTINGS = { company: "Stream-Net", currency: "EGP", mode: MODE, expired_pool: "END", quota_pool: "LIMIT", expiring_days: "3", sync_minutes: "15", fast_seconds: "20", default_days: "30",
    mac_bind: "0", wa_number: phone(1), wa_gap: "8", goip_enabled: "0", goip_ip: "192.0.2.50", goip_line: "1", loyalty_on: "1", loyalty_for: "subscriber", com_phone: phone(2), address: "عنوان تجريبي",
    portal_on: "1", card_templates: JSON.stringify([{ id: "t1", name: "قالب تجربة", w: 85, h: 54, cols: 3, rows: 7, gap: 2, bg: "", color: "#4f46e5",
      f: { code: { x: 50, y: 48, s: 18, c: "#000000", on: true }, price: { x: 85, y: 15, s: 12, c: "#1e3a8a", on: true }, profile: { x: 18, y: 15, s: 11, c: "#1e3a8a", on: true },
        series: { x: 18, y: 88, s: 9, c: "#475569", on: true }, company: { x: 50, y: 15, s: 13, c: "#0f172a", on: false } }, qr: { on: true, x: 85, y: 70, s: 16 } }]) };
  let nextId = 5000;

  // ---------- derived values
  const stOf = s => {
    if (!+s.enabled) return "_dis_";
    const e = parseDT(s.expiration);
    if (!e || e < NOW) return "_exp_";
    if (+s.profile_quota > 0 && +s.used_mb >= +s.quota_mb) return "_exp_q";
    if (e - NOW < DAY) return "_exp_1";
    if (e - NOW < 3 * DAY) return "_exp_3";
    return "_act_";
  };
  const subRow = s => ({ ...s, st: stOf(s), nas: s.nas_name, _since: undefined, _in: undefined, _out: undefined });
  const counts = () => {
    const c = { all: SUBS.length, active: 0, expired: 0, disabled: 0, expiring: 0, online: 0 };
    SUBS.forEach(s => { const e = parseDT(s.expiration); if (!+s.enabled) c.disabled++; else if (!e || e < NOW) c.expired++; else { c.active++; if (e - NOW < 3 * DAY) c.expiring++; } if (+s.online) c.online++; });
    return c;
  };
  const onlineAdv = () => SUBS.filter(s => +s.online).map(s => { const p = prof(s.profile_id), m = mgr(s.manager_id);
    return { sid: s.id, username: s.username, firstname: s.firstname, lastname: s.lastname, profile: s.profile, ip: s.ip, nas: s.nas, nas_name: s.nas_name, down_speed: p.down_speed, up_speed: p.up_speed,
      session_sec: Math.floor((NOW - s._since) / 1000), expiration: s.expiration, bytes_in: s._in, bytes_out: s._out, mac: s.mac, manager: s.manager, created_by: m ? m.adv_id : "", serv_name: "pppoe-main", portid: "vlan10" }; })
    .concat([0, 1, 2].map(k => ({ sid: 0, username: String(70001000 + k), firstname: "", lastname: "", profile: "Cafe-2H", ip: `10.0.0.${120 + k}`, nas: NAS[1].ip, nas_name: NAS[1].name,
      down_speed: 30720, up_speed: 7680, session_sec: ri(300, 6000), expiration: "", bytes_in: ri(20, 200) * 1048576, bytes_out: ri(100, 900) * 1048576, mac: mac(800 + k), manager: "samy.west", created_by: 13, serv_name: "hs-cafe", portid: "" })));
  const onlineOwn = () => SUBS.filter(s => +s.online).map(s => ({ username: s.username, nasipaddress: s.nas, framedipaddress: s.ip, callingstationid: s.mac, acctstarttime: dt(s._since),
    acctinputoctets: s._in, acctoutputoctets: s._out }));
  const usageDays = n => Array.from({ length: n }, (_, i) => { const d = n - 1 - i; return { day: dOnly(ago(d)), down: Math.round((380 + 90 * Math.sin(d / 2) + (d % 7 === 5 ? 120 : 0)) * GBB), up: Math.round((52 + 12 * Math.cos(d / 3)) * GBB) }; });
  const invSum = (kind) => { const a = INV.filter(x => x.kind === kind), v = x => x.amount - x.discount, today = dOnly(NOW), month = dOnly(NOW).slice(0, 7);
    return { paid: a.filter(x => +x.paid).reduce((s, x) => s + v(x), 0), unpaid: a.filter(x => !+x.paid).reduce((s, x) => s + v(x), 0), count: a.length,
      today: a.filter(x => x.created.startsWith(today) && +x.paid).reduce((s, x) => s + v(x), 0), month: a.filter(x => x.created.startsWith(month) && +x.paid).reduce((s, x) => s + v(x), 0) }; };
  const advMetrics = () => { const c = counts(), is = invSum("subscriber"), cs = invSum("card");
    return { quick_stats: { online_users: c.online, today_sales_total: is.today, today_sales_count: INV.filter(x => x.kind === "subscriber" && x.created.startsWith(dOnly(NOW))).length, unpaid_total: is.unpaid, unpaid_invoices_count: INV.filter(x => !+x.paid).length },
      get_cards_sales_stats: { day: { total: cs.today, count: 1 }, month: { total: cs.month, count: 9 }, year: { total: cs.paid + 3400, count: 152 } }, borrowed_total: SUBS.filter(s => +s.borrowed).length * 320 }; };
  const quote = s => { const p = prof(s.profile_id), m = mgr(s.manager_id);
    return { username: s.username, expiration: s.expiration, profile: s.profile, price: p.price || 0, pquota: p.quota_mb || 0, quota_mb: s.quota_mb, used_mb: s.used_mb,
      manager_balance: m ? (+m.unlimited ? "غير محدود" : m.balance) : "غير محدود", debt: s.unpaid_total, advance: s.borrowed ? p.price : 0, money: s.money }; };
  const extendSub = (s, ms) => { const base = Math.max(NOW, parseDT(s.expiration) || 0); s.expiration = dt(base + ms); return s.expiration; };
  const addInvoice = (s, op, paid = 1) => { const p = prof(s.profile_id); INV.unshift({ id: invId++, sub_id: s.id, created: dt(Date.now()), kind: "subscriber", username: s.username, manager: s.manager, profile: p.name,
    operation: op, amount: p.price || 0, discount: 0, paid: paid ? 1 : 0, created_by: "admin", period_from: dt(Date.now()), period_to: s.expiration, notes: paid ? "" : "سلفة" }); };

  // ---------- GET answers
  function subList(q) {
    let a = SUBS.slice();
    const st = q.get("status") || "", text = (q.get("q") || "").trim().toLowerCase(), size = +q.get("size") || 25, page = Math.max(1, +q.get("page") || 1);
    if (text) a = a.filter(x => [x.username, x.firstname, x.lastname, x.mobile, x.ip, x.mac, x.notes, x.address].join(" ").toLowerCase().includes(text));
    if (+q.get("profile_id")) a = a.filter(x => x.profile_id === +q.get("profile_id"));
    if (+q.get("manager_id")) a = a.filter(x => x.manager_id === +q.get("manager_id"));
    const by = q.get("date_by"), from = q.get("from"), to = q.get("to");
    if (by && (from || to)) a = a.filter(x => inRange(parseDT(x[by === "expiration" ? "expiration" : "created"]), from, to));
    const F = { active: x => stOf(x) !== "_exp_" && stOf(x) !== "_dis_", expired: x => stOf(x) === "_exp_", quota: x => stOf(x) === "_exp_q", expiring: x => ["_exp_1", "_exp_3"].includes(stOf(x)),
      today: x => stOf(x) === "_exp_1", unavailable: x => ["_exp_", "_dis_", "_exp_q"].includes(stOf(x)), offline: x => !+x.online, online: x => +x.online, borrowed: x => +x.borrowed,
      auto_sub: x => +x.auto_renew, auto_mgr: x => +x.auto_renew, static: x => !!x.framed_ip, debtors: x => +x.unpaid_total > 0, discount: x => +x.discount > 0, disabled: x => !+x.enabled,
      nosession: x => !+x.online, special_speed: () => false, temp_speed: () => false, none: () => false };
    if (F[st]) a = a.filter(F[st]);
    const sort = q.get("sort");
    const S = { name: (x, y) => x.username.localeCompare(y.username), money: (x, y) => y.money - x.money, quota: (x, y) => (y.quota_mb - y.used_mb) - (x.quota_mb - x.used_mb),
      exp: (x, y) => parseDT(x.expiration) - parseDT(y.expiration), exp_desc: (x, y) => parseDT(y.expiration) - parseDT(x.expiration), created: (x, y) => parseDT(y.created) - parseDT(x.created) };
    if (S[sort]) a.sort(S[sort]);
    return { rows: a.slice((page - 1) * size, page * size).map(subRow), total: a.length, all: SUBS.length };
  }
  function subDetail(s) {
    const p = prof(s.profile_id);
    const sessions = Array.from({ length: 5 }, (_, k) => { const st = ago(k + 1, ri(0, 10)); return { acctstarttime: dt(st), acctstoptime: dt(st + ri(2, 20) * HOUR), nasipaddress: s.nas || NAS[0].ip,
      framedipaddress: `10.0.0.${20 + (s.id % 60)}`, callingstationid: s.mac, acctinputoctets: ri(1, 20) * GBB / 10, acctoutputoctets: ri(5, 60) * GBB / 4 }; });
    if (+s.online) sessions.unshift({ acctstarttime: dt(s._since), acctstoptime: "", nasipaddress: s.nas, framedipaddress: s.ip, callingstationid: s.mac, acctinputoctets: s._in, acctoutputoctets: s._out });
    return { subscriber: subRow(s), invoices: INV.filter(x => x.sub_id === s.id), sessions,
      adv_online: +s.online ? [{ nas: s.nas_name, ip: s.ip, mac: s.mac, since: dt(s._since) }] : [],
      radius: [{ t: "check", attribute: "Cleartext-Password", op: ":=", value: "••••••" }, { t: "check", attribute: "Simultaneous-Use", op: ":=", value: String(s.simul) },
        { t: "reply", attribute: "Mikrotik-Rate-Limit", op: ":=", value: p.rate_limit || "" }, { t: "reply", attribute: "Framed-Pool", op: ":=", value: p.pool || "" }]
        .concat(s.framed_ip ? [{ t: "reply", attribute: "Framed-IP-Address", op: ":=", value: s.framed_ip }] : []),
      log: AUDIT.filter(x => x.target === s.username).concat([{ ts: s.created, admin: s.manager, action: "create" }]) };
  }
  function salesReport(q) {
    const from = q.get("from") || dOnly(ago(30)), to = q.get("to") || dOnly(NOW), g = q.get("group") || "day", m = {};
    INV.filter(x => inRange(parseDT(x.created), from, to)).forEach(x => {
      const k = g === "month" ? x.created.slice(0, 7) : g === "manager" ? x.manager || x.created_by : g === "profile" ? x.profile || "—" : g === "kind" ? x.kind : x.created.slice(0, 10);
      const r = m[k] = m[k] || { k, n: 0, paid: 0, unpaid: 0 }; r.n++; r[+x.paid ? "paid" : "unpaid"] += x.amount - x.discount;
    });
    return { from, to, rows: Object.values(m).sort((a, b) => String(b.k).localeCompare(String(a.k))) };
  }
  function dailyUsage(q) {
    const from = parseDT((q.get("from") || dOnly(ago(30))) + " 00:00:00"), to = parseDT((q.get("to") || dOnly(NOW)) + " 00:00:00"), data = [];
    for (let t = from; t <= to && data.length < 120; t += DAY) { const dl = ri(2, 18) * GBB / 2, ul = ri(1, 6) * GBB / 4; data.push({ day: dOnly(t), download: dl, upload: ul, total: dl + ul }); }
    return { data };
  }
  function localReport(name) {
    const S = i => SUBS[(i * 7) % SUBS.length];
    const gen = {
      login_states: () => SUBS.slice(0, 12).map((s, i) => ({ username: s.username, status: i % 4 ? "Access-Accept" : "Access-Reject", ip: s.ip || "—", t_date: dt(ago(0, i)) })),
      manager_sales_local: () => MGRS.map(m => ({ manager: m.username, count: INV.filter(x => x.manager === m.username).length, total: INV.filter(x => x.manager === m.username && +x.paid).reduce((a, x) => a + x.amount, 0) })),
      disconnects: () => Array.from({ length: 8 }, (_, i) => ({ t_date: dt(ago(0, i * 3)), username: S(i).username, action: i % 2 ? "disconnect" : "coa-rate", nas: NAS[i % 2].name, status: i === 5 ? "فشل" : "تم" })),
      cash: () => CASH.filter(c => c.status === "used").map(c => ({ t_date: c.tx_time.replace("T", " "), username: c.used_for, total: c.amount, operation: c.source_text })),
      cash_details: () => CASH.map(c => ({ id: c.id, t_date: c.tx_time.replace("T", " "), name: c.sender_name || c.wallet, total: c.amount, status: c.status, username: c.used_for })),
      mac_changes: () => Array.from({ length: 6 }, (_, i) => ({ t_date: dt(ago(i, 2)), username: S(i).username, more_details: `${mac(700 + i)} ← ${S(i).mac}`, manager: "admin" })),
      profile_changes: () => Array.from({ length: 6 }, (_, i) => ({ t_date: dt(ago(i * 2, 1)), username: S(i + 3).username, more_details: `${PROFS[i % 4].name} ← ${PROFS[(i % 4) + 1].name}`, manager: S(i).manager })),
      balance_transfers: () => Array.from({ length: 4 }, (_, i) => ({ t_date: dt(ago(i * 3)), username: S(i).username, more_details: "إلى " + S(i + 5).username, total: ri(2, 10) * 10 })),
      login_failed: () => Array.from({ length: 5 }, (_, i) => ({ t_date: dt(ago(i, 4)), username: i % 2 ? "admin" : "unknown", ip: `192.0.2.${200 + i}` })),
      traffic_top: () => SUBS.slice(0, 15).map((s, i) => ({ username: s.username, down_gb: 180 - i * 9, up_gb: 25 - i, total_gb: 205 - i * 10 })),
      traffic_monthly: () => [3, 2, 1, 0].map(k => { const d = new Date(); d.setMonth(d.getMonth() - k); return { month: `${d.getFullYear()}-${p2(d.getMonth() + 1)}`, down_gb: 11000 + k * 400, up_gb: 1500 + k * 60, users: 40 }; }),
      traffic_daily: () => usageDays(30).map(d => ({ day: d.day, down_gb: (d.down / GBB).toFixed(1), up_gb: (d.up / GBB).toFixed(1) })),
      debtors: () => SUBS.filter(s => +s.unpaid_total > 0).map(s => ({ username: s.username, full_name: `${s.firstname} ${s.lastname}`, mobile: s.mobile, total: s.unpaid_total, manager: s.manager })),
      radius_sessions: () => onlineOwn(),
      radius_raw: () => SUBS.slice(0, 10).map((s, i) => ({ t_date: dt(ago(0, 0, i * 7)), username: s.username, status: i === 3 ? "Access-Reject" : "Access-Accept", nas: NAS[i % 2].ip })),
      aging: () => [{ manager: "nour.center", d0_30: 960, d31_60: 320, d61_90: 0, d90_plus: 0 }, { manager: "hassan.market", d0_30: 355, d31_60: 0, d61_90: 400, d90_plus: 260 }, { manager: "samy.west", d0_30: 0, d31_60: 0, d61_90: 0, d90_plus: 0 }],
    };
    return { data: gen[name] ? gen[name]() : [] };
  }
  function advReport(ep) {
    const inv = INV.filter(x => x.kind === "subscriber");
    const gen = {
      get_activity_log: () => ({ data: AUDIT.map((a, i) => ({ id: i + 1, username: a.admin, t_date: unix(parseDT(a.ts)), operation: a.action, more_details: a.target })) }),
      report_manager_sales: () => ({ data: INV.filter(x => x.kind === "manager").map(x => ({ id: x.id, user_manager: x.manager, date_operation: unix(parseDT(x.created)), operation: "إيداع", new_price: x.amount, paid_unpaid: x.paid, notes: x.notes })) }),
      report_summary: () => ({ paid: invSum("subscriber").paid, unpaid: invSum("subscriber").unpaid, count: inv.length, discount: inv.reduce((a, x) => a + +x.discount, 0) }),
      get_cards_sales_stats: () => advMetrics().get_cards_sales_stats,
      get_manager_balance_log: () => ({ data: (LEDGER[1] || []).map(r => ({ t_date: unix(parseDT(r.created)), user_manager: "nour.center", operation: r.reason, new_price: r.delta, old_price: r.balance_after - r.delta, notes: r.reason })) }),
      report_invoices: () => ({ sum_paid: invSum("subscriber").paid, sum_unpaid: invSum("subscriber").unpaid, data: inv.map(x => ({ id: x.id, username: x.username, date_operation: unix(parseDT(x.created)), operation: x.operation,
        amount: x.amount, new_price: x.amount, discount: x.discount, paid_unpaid: x.paid, user_manager: x.manager })) }),
      get_user_sessions: () => ({ data: subDetail(SUBS[0]).sessions }),
      report_user_usage: () => ({ data: SUBS.slice(0, 12).map((s, i) => ({ username: s.username, down_gb: 120 - i * 7, up_gb: 18 - i, total_gb: 138 - i * 8 })) }),
      quick_stats: () => advMetrics().quick_stats,
      report_nas_usage: () => ({ data: NAS.map((n, i) => ({ name: n.name, ip: n.ip, down_gb: 9000 - i * 5000, up_gb: 1200 - i * 700, users: onlineAdv().filter(o => o.nas === n.ip).length })) }),
      get_dashboard_metrics: () => ({ subscribers: counts(), sales: invSum("subscriber"), cards: invSum("card") }),
      get_tickets: () => ({ data: TICKETS.map(t => ({ id: t.id, username: t.username, title: t.subject, status: t.status, last_reply_by: (REPLIES[t.id] || []).slice(-1)[0]?.created_by || t.created_by, replies_count: (REPLIES[t.id] || []).length })) }),
    };
    return gen[ep] ? gen[ep]() : { data: [] };
  }
  function cashRows(q) {
    const from = q.get("from"), to = q.get("to"), text = (q.get("q") || "").trim().toLowerCase();
    return CASH.filter(c => inRange(parseDT(c.tx_time), from, to) && (!text || [c.code, c.wallet, c.sender_name, c.amount, c.used_for].join(" ").toLowerCase().includes(text)))
      .sort((a, b) => parseDT(b.tx_time) - parseDT(a.tx_time));
  }
  function cashAdv() {
    const ip = CASH.filter(c => c.source === "instapay" || c.source === "ipnotif");
    return { matched: ip.slice(0, 5).map((c, i) => ({ adv_code: String(61000 + i), code: c.code, wallet: c.wallet, name: c.sender_name, amount: c.amount, our_at: c.tx_time.replace("T", " ").slice(0, 16),
        how: ["address", "name", "guess"][i % 3], adv_used: c.status === "used", adv_by: c.used_by, status: c.status })),
      missing: [{ adv_code: "61090", wallet: "demo90@instapay", amount: 355, at: dt(ago(0, 5)).slice(0, 16), adv_used: true, adv_by: "admin", before_app: false },
        { adv_code: "61091", wallet: "demo91@instapay", amount: 320, at: dt(ago(1, 20)).slice(0, 16), adv_used: false, adv_by: "", before_app: true }],
      extra: ip.slice(5, 6).map(c => ({ code: c.code, name: c.sender_name, wallet: c.wallet, amount: c.amount, at: c.tx_time.replace("T", " ").slice(0, 16), status: c.status })),
      imported: iso(ago(0, 3)), offset: 3, cover: `${dOnly(ago(1))} → ${dOnly(NOW)}`, outside: 2 };
  }
  const sysinfo = () => ({ cpu: "23%", ram: "41%", disk: "37%", uptime: "12 يوم 4 ساعة", ping_radius: { ms: "1.2 ms", loss: "0%" }, ping_google: { ms: "18 ms", loss: "0%" }, now: dt(Date.now()) });
  const dashboard = () => { const c = counts(), on = c.online;
    return { mode: MODE, online: on, adv_online: on, subscribers: c.all, active: c.active, expired: c.expired, disabled: c.disabled, expiring: c.expiring, expiring_days: 3,
      received: INV.filter(x => +x.paid).reduce((a, x) => a + x.amount - x.discount, 0), balance: "غير محدود", cards_online: 3, cards: CARDS.length + 150, cards_used: CARDS.filter(x => x.status === "used").length + 114,
      inv_subscribers: invSum("subscriber"), inv_cards: invSum("card"), inv_managers: invSum("manager"), radius_ok: true, db_ok: true, uptime: "12 يوم 4 ساعة", now: dt(Date.now()), version: "demo-1.0",
      last_sync: dt(ago(0, 0, 4)), sync_state: "تمام", nas_active: 2, nas_total: NAS.length, borrowed_total: advMetrics().borrowed_total, usage_days: usageDays(14) }; };

  function GET(path, q) {
    const seg = path.split("/");
    switch (path) {
      case "me": return { admin: { id: 1, username: "admin", full_name: "مدير التجربة", role: "admin", perms: null }, company: SETTINGS.company, mode: MODE, sim: false };
      case "dashboard": return dashboard();
      case "sysinfo": return sysinfo();
      case "subscribers": return subList(q);
      case "profiles": case "profiles/full": return PROFS.map(p => ({ ...p, users: SUBS.filter(s => s.profile_id === p.id).length }));
      case "managers": return MGRS.map(m => ({ ...m, users: SUBS.filter(s => s.manager_id === m.id).length }));
      case "online": return { own: MODE === "live" ? onlineOwn() : [], adv: onlineAdv() };
      case "online-managers": return [{ token: "tk1", username: "admin", full_name: "مدير التجربة", me: true, online: true, last_action: "dashboard", last_seen: dt(Date.now()), created: dt(ago(0, 2)), ip: "10.0.0.150", browser: "Chrome", os: "Windows", ua: "Mozilla/5.0 (demo)" },
        { token: "tk2", username: "nour.center", full_name: "نور الدين عبد الغني", me: false, online: true, last_action: "subscribers", last_seen: dt(ago(0, 0, 2)), created: dt(ago(0, 5)), ip: "10.0.0.151", browser: "Chrome Mobile", os: "Android", ua: "Mozilla/5.0 (demo)" },
        { token: "tk3", username: "support", full_name: "خدمة العملاء", me: false, online: false, last_action: "online", last_seen: dt(ago(0, 3)), created: dt(ago(0, 9)), ip: "10.0.0.152", browser: "Firefox", os: "Windows", ua: "Mozilla/5.0 (demo)" }];
      case "invoices": { const k = q.get("kind"), pd = q.get("paid"), from = q.get("from"), to = q.get("to");
        return INV.filter(x => (!k || x.kind === k) && (pd === null || pd === "" || +x.paid === +pd) && inRange(parseDT(x.created), from, to)); }
      case "reports/sales": return salesReport(q);
      case "reports/local": return localReport(q.get("name"));
      case "reports/daily-usage": return dailyUsage(q);
      case "cards": { const k = q.get("kind"), st = q.get("status"), text = (q.get("q") || "").toLowerCase(), se = q.get("series");
        const rows = CARDS.filter(c => (!k || c.kind === k) && (!st || c.status === st) && (!se || c.series === se) && (!text || (c.code + " " + c.series).toLowerCase().includes(text)));
        const stats = {}; CARDS.forEach(c => { const key = c.kind + c.status; (stats[key] = stats[key] || { kind: c.kind, status: c.status, n: 0 }).n++; });
        return { stats: Object.values(stats), rows }; }
      case "cardseries": return CARD_SERIES.map((r, i) => ({ id: i + 1, raw: JSON.stringify(r) }));
      case "cafe": {
        if (q.get("online")) return cafeCards(1).filter(c => +c.online).map((c, k) => ({ username: c.username, cafe: "كافيه النخيل", series: "نخيل-2س", profile: "30M", framedipaddress: `10.0.0.${120 + k}`,
          callingstationid: c.mac, uptime: `${ri(5, 90)} دقيقة`, down_mb: ri(50, 700), up_mb: ri(5, 80), expires: c.expires }));
        if (q.get("report")) return CAFE_SERIES.map(s => ({ name: s.name, cafe: s.cafe, profile: s.profile, qty: s.qty, used: s.qty - s.available, available: s.available, active: s.active,
          price: s.price, cost: s.cost, sales: (s.qty - s.available) * s.price, profit: (s.qty - s.available) * (s.price - s.cost), created: s.created }));
        if (q.get("series")) return { cards: cafeCards(q.get("series")) };
        if (q.get("card")) { const c = cafeCards(1)[0], v = q.get("card");
          return { card: { username: v, status: "active", cafe: "كافيه النخيل", series: "نخيل-2س", profile: "30M", first_login: c.first_login, expires: c.expires, quota_mb: 2048, used_bytes: 412 * 1048576, used_secs: 3900, macs: mac(901) + "," + mac(902), mac_max: 2 },
            sessions: [0, 1].map(k => ({ acctstarttime: dt(ago(0, 3 - k)), acctstoptime: k ? "" : dt(ago(0, 2)), duration: k ? "—" : "60 دقيقة", down_mb: ri(50, 300), up_mb: ri(5, 30), callingstationid: mac(901 + k), login: k ? mac(902) : v, acctterminatecause: k ? "" : "User-Request" })),
            events: [{ ts: dt(ago(0, 2)), reason_text: "المشترك قفل الاتصال", detail: "User-Request" }] }; }
        if (q.get("scripts")) return { hotspot: "# demo hotspot settings\n/ip hotspot profile set [find] login-by=http-chap\n", watch: "# demo watchdog\n/system scheduler add name=demo-watch interval=1m\n", users: "# demo users file\n", cards: 36 };
        return { series: CAFE_SERIES, cafes: CAFES.map(c => ({ id: c.id, name: c.name, ip: c.ip })) };
      }
      case "nas": return { local: NAS.map(n => ({ id: n.id, nasname: n.ip, shortname: n.name, type: n.type, description: n.description })),
        adv: NAS.map(n => ({ id: n.id, name: n.name, ip: n.ip, type: n.type, description: n.description, online: onlineAdv().filter(o => o.nas === n.ip).length })) };
      case "tickets": return TICKETS.map(({ body, ...t }) => t);
      case "messages": return MESSAGES;
      case "settings": return { settings: Object.entries(SETTINGS).map(([k, v]) => ({ k, v })), default_password_set: true, host: "demo" };
      case "speed": { const mul = SPEED.mode === "off" ? 1 : SPEED.value;
        return { ...SPEED, live: MODE === "live", note: "(وضع تجربة — مفيش راوتر)", profiles: PROFS.filter(p => +p.enabled).map(p => { const m = p.down_speed / 1024, now = SPEED.type === "add" ? m + (SPEED.mode === "off" ? 0 : SPEED.value) : m * mul;
          return { name: p.name, users: SUBS.filter(s => s.profile_id === p.id).length, base: `${m / 4}M/${m}M`, now: `${now / 4}M/${now}M` }; }) }; }
      case "sync": { const c = counts();
        return { state: "تمام", last: dt(ago(0, 0, 4)), local: { subscribers: c.all, from_adv: SUBS.filter(s => s.source === "adv").length, profiles: PROFS.length, managers: MGRS.length, cards: CARDS.length, adv_online: c.online, no_profile: 0, no_expiration: 0, radius_users: c.all },
          adv_metrics: advMetrics(), logs: Array.from({ length: 6 }, (_, i) => ({ ts: dt(ago(0, 0, 4 + i * 15)), kind: ["subscribers", "online", "profiles"][i % 3], ok: i === 4 ? 0 : 1, fetched: ri(10, 60), changed: ri(0, 5), message: i === 4 ? "timeout (تجربة)" : "" })) }; }
      case "sync/quota-compare": return { running: false, state: "", last: { ts: dt(ago(1, 2)), minutes: 60, users: 12, skipped: 1, adv_mb: 41200, down_mb: 37100, up_mb: 4300, vs_down_pc: -10, vs_up_pc: -89.6, vs_total_pc: 0.5,
        rows: SUBS.slice(0, 6).map((s, i) => ({ user: s.username, adv_mb: 3000 + i * 400, down_mb: 2700 + i * 360, up_mb: 300 + i * 40, diff_pc: i % 2 ? 0.2 : -0.4 })) } };
      case "sync/verify": return { running: false, last: { ts: dt(ago(0, 5)), checked: 100, identical: 98, different: 2, fixed: 1, diffs: [{ user: SUBS[4].username, field: "expiration", adv: SUBS[4].expiration, ours: SUBS[4].expiration.slice(0, 16) }, { user: SUBS[9].username, field: "mobile", adv: phone(9999), ours: SUBS[9].mobile }] } };
      case "sync/archive": return { running: false, state: "خلصت (تجربة)", activity: 12840, sessions: 96311 };
      case "trash": return TRASH;
      case "audit": return AUDIT;
      case "admins": return ADMINS;
      case "security-groups": return [{ name: "موزعين", managers: 2, perms: 34, created: dOnly(ago(200)) }, { name: "كافيهات", managers: 1, perms: 12, created: dOnly(ago(90)) }, { name: "مشاهدة بس", managers: 0, perms: 5, created: dOnly(ago(30)) }];
      case "maintenance": if (q.get("script")) return { script: "# أوامر تجربة — مش حقيقية\n/radius add address=192.0.2.10 secret=DEMO service=ppp\n/ppp aaa set use-radius=yes\n" };
        return { radius: true, fail2ban: true, ssh: true, sessions: counts().online, version: "demo-1.0" };
      case "backups": return BACKUPS;
      case "vpn": if (q.get("script")) return { script: "# أوامر تجربة للكافيه — مش حقيقية\n/interface sstp-client add name=sstp-demo connect-to=vpn.example.com user=cafe-demo password=DEMO\n" };
        return { running: true, installed: true, host: "vpn.example.com", port: "4444", net: "10.0.0.192/26", clients: CAFES };
      case "wa/status": return { state: "connected", number: phone(1), me: "20" + phone(1).slice(1), queued: 0, sent_today: 12, failed_today: 1 };
      case "cash": {
        if (q.get("devices")) return { devices: DEVICES, points: { from: dOnly(ago(20)), vf: CASH.filter(c => c.source === "vfcash").length, ip: 2 * CASH.filter(c => /instapay|ipnotif/.test(c.source)).length, used: 0, start: 5000, left: 0 },
          app: "1.4", senders: "VF-Cash,DemoBank", apps: "com.demo.instapay", auto: true, noauto: phone(5) };
        if (q.get("raw")) return CASH_UNKNOWN;
        return { rows: cashRows(q), unknown: CASH_UNKNOWN.length };
      }
      case "cash/adv": return cashAdv();
      case "cash/find": { const v = (q.get("q") || "").toLowerCase(); return CASH.filter(c => [c.code, c.wallet, c.sender_name, c.amount].join(" ").toLowerCase().includes(v)).slice(0, 8); }
    }
    // dynamic paths
    if (seg[0] === "subscribers" && seg[1]) {
      const s = sub(seg[1]); if (!s) return { error: "المشترك مش موجود", _status: 404 };
      if (!seg[2] || seg[2] === "full") return subDetail(s);
      if (seg[2] === "prices") return PROFS.filter(p => +p.enabled).map(p => ({ name: p.name, base: p.price, price: s.manager_id === 2 ? p.price - 10 : p.price }));
      if (seg[2] === "quote") return quote(s);
      if (seg[2] === "adv-sessions") return Array.from({ length: 4 }, (_, k) => { const st = ago(40 + k * 3); return { start: dt(st), stop: dt(st + 9 * HOUR), bytes_out: ri(5, 50) * GBB / 4, bytes_in: ri(1, 10) * GBB / 10, nas: NAS[0].name, ip: `10.0.0.${60 + k}`, mac: s.mac }; });
    }
    if (seg[0] === "managers" && seg[2] === "balance") return LEDGER[+seg[1]] || [];
    if (seg[0] === "tickets" && seg[1]) { const t = TICKETS.find(x => x.id === +seg[1]); return t ? { ticket: [t], replies: [{ created_by: t.created_by, created: t.created, body: t.body }].concat(REPLIES[t.id] || []) } : { ticket: [], replies: [] }; }
    if (seg[0] === "adv") return advReport(seg[1]);
    return [];
  }

  // ---------- POST / PUT / DELETE answers (update the data where it is easy, never fail)
  function WRITE(method, path, q, b) {
    b = b || {};
    const seg = path.split("/");
    if (path === "login" || path === "logout") return { ok: true };
    if (path === "settings" && method === "PUT") {
      Object.entries(b).forEach(([k, v]) => { if (k !== "confirm") SETTINGS[k] = String(v); });
      if (b.mode) { MODE = b.mode === "live" ? "live" : "shadow"; try { sessionStorage.setItem("rm_demo_mode", MODE); } catch (_) {} }
      return { ok: true };
    }
    if (seg[0] === "subscribers") {
      if (path === "subscribers" || path === "subscribers/full") {
        const p = prof(b.ProfileID), m = mgr(b.ManagerID), id = Math.max(...SUBS.map(s => s.id)) + 1;
        SUBS.unshift({ id, adv_id: 0, username: b.Username || "new" + id, firstname: b.Firstname || "", lastname: b.Lastname || "", mobile: b.Mobile || "", address: b.Address || "", notes: b.Notes || "",
          profile_id: p.id || 0, profile: p.name || "", profile_quota: p.quota_mb || 0, quota_mb: b.QuotaMB ?? p.quota_mb ?? 0, used_mb: 0, manager_id: m ? m.id : 0, manager: m ? m.username : "",
          expiration: b.Expiration || "", created: dt(Date.now()), enabled: b.Enabled === false ? 0 : 1, online: 0, ip: "", nas: "", nas_name: "", mac: b.Mac || "", framed_ip: b.FramedIP || "",
          money: 0, paid_total: 0, unpaid_total: 0, discount: 0, borrowed: 0, simul: b.Simul || 1, auto_renew: b.AutoRenew ? 1 : 0, source: "local", extra: JSON.stringify(b.Extra || {}) });
        return { ok: true, id };
      }
      if (seg[1] === "bulk") {
        (b.IDs || []).map(sub).filter(Boolean).forEach(s => {
          if (b.Action === "toggle") s.enabled = +s.enabled ? 0 : 1;
          if (b.Action === "disable") s.enabled = 0;
          if (b.Action === "extend") extendSub(s, (+b.Days || 1) * DAY);
          if (b.Action === "profile" && prof(b.ProfileID).id) { s.profile_id = +b.ProfileID; s.profile = prof(b.ProfileID).name; }
          if (b.Action === "mac") s.mac = b.Value || s.mac; if (b.Action === "unmac") s.mac = ""; if (b.Action === "ip") s.framed_ip = b.Value || "";
        });
        return { ok: true, done: (b.IDs || []).length };
      }
      if (seg[1] === "genadj") return { ok: true, count: 3, n: 3 };
      const s = sub(seg[1]); if (!s) return { ok: true };
      if (method === "PUT") {
        const p = prof(b.ProfileID), m = mgr(b.ManagerID);
        Object.assign(s, { firstname: b.Firstname ?? s.firstname, lastname: b.Lastname ?? s.lastname, mobile: b.Mobile ?? s.mobile, address: b.Address ?? s.address, notes: b.Notes ?? s.notes,
          mac: b.Mac ?? s.mac, framed_ip: b.FramedIP ?? s.framed_ip, expiration: b.Expiration ?? s.expiration, simul: b.Simul || s.simul });
        if (p.id) { s.profile_id = p.id; s.profile = p.name; s.profile_quota = p.quota_mb; }
        if (m) { s.manager_id = m.id; s.manager = m.username; }
        if (b.Enabled !== undefined) s.enabled = b.Enabled ? 1 : 0;
        if (b.AutoRenew !== undefined) s.auto_renew = b.AutoRenew ? 1 : 0;
        if (b.Extra) s.extra = JSON.stringify(b.Extra);
        if (b.QuotaMB !== undefined && b.QuotaMB !== null) s.quota_mb = +b.QuotaMB;
        return { ok: true };
      }
      if (seg[2] === "toggle") { s.enabled = +s.enabled ? 0 : 1; return { ok: true }; }
      if (seg[2] === "extend") return { ok: true, expiration: extendSub(s, (+b.Days || 1) * DAY) };
      if (seg[2] === "renew") { if (prof(b.ProfileID).id) { s.profile_id = +b.ProfileID; s.profile = prof(b.ProfileID).name; } const e = extendSub(s, 30 * DAY); addInvoice(s, "renew", b.Paid !== false); return { ok: true, expiration: e }; }
      if (seg[2] === "op") {
        const a = b.Action, unitMs = { 1: DAY, 2: HOUR, 3: 6e4 }[+b.Unit] || DAY;
        if (a === "activate" || a === "charge_credit") { const e = extendSub(s, 30 * DAY * Math.max(1, +b.Count || 1)); addInvoice(s, "activate", b.Paid !== false); return { ok: true, expiration: e }; }
        if (a === "add_time" || a === "add_ch" || a === "self_borrow") { if (a !== "add_time") s.borrowed = 1; return { ok: true, expiration: extendSub(s, (+b.Value || 1) * unitMs) }; }
        if (a === "charge_money") { s.money = +s.money + (+b.Amount || 0); return { ok: true, money: s.money }; }
        if (a === "withdraw_money") { s.money = Math.max(0, +s.money - (+b.Amount || 0)); return { ok: true, money: s.money }; }
        if (a === "part_pay") { s.unpaid_total = Math.max(0, +s.unpaid_total - (+b.Amount || 0)); return { ok: true }; }
        if (a === "change_profile") { const p = prof(b.ProfileID); if (p.id) { s.profile_id = p.id; s.profile = p.name; } return { ok: true }; }
        if (a === "rename" && b.Target) { s.username = String(b.Target); return { ok: true }; }
        if (a === "delete") { SUBS.splice(SUBS.indexOf(s), 1); return { ok: true }; }
        if (a === "disconnect") { s.online = 0; return { ok: true }; }
        if (a === "trx") { const c = CASH.find(x => x.code === String(b.Target)), amount = c ? +c.amount : 0;
          if (c) Object.assign(c, { status: "used", used_for: s.username, used_at: iso(Date.now()), used_by: "admin" });
          s.money = +s.money + amount; let expiration = "";
          if (+b.Count && s.money >= (prof(s.profile_id).price || 0)) { s.money -= prof(s.profile_id).price || 0; expiration = extendSub(s, 30 * DAY); addInvoice(s, "renew"); }
          return { ok: true, amount, money: s.money, expiration, note: expiration ? "" : "الرصيد مش كفاية للتجديد (تجربة)" }; }
        if (a === "send_msg") return { ok: true, note: "وضع تجربة — مفيش رسالة اتبعتت" };
        return { ok: true };
      }
      return { ok: true };
    }
    if (seg[0] === "profiles") {
      const f = b, name = f.profile_name || f.Name;
      const p = seg[1] && seg[1] !== "full" ? prof(seg[1]) : null;
      const vals = { name: name || (p && p.name) || "عرض جديد", price: +(f.price ?? f.Price ?? 0), down_speed: +(f.down_speed ?? f.DownSpeed ?? 0) || 0, up_speed: +(f.up_speed ?? f.UpSpeed ?? 0) || 0, quota_mb: +(f.profile_qouta ?? f.QuotaMB ?? 0) || 0, pool: f.pool_name || f.Pool || "" };
      if (p && p.id) { Object.assign(p, vals); p.f = { ...p.f, ...f }; return { ok: true, id: p.id }; }
      const id = Math.max(...PROFS.map(x => x.id)) + 1; PROFS.push({ id, adv_id: 100 + id, ...vals, rate_limit: "", days: 30, simul: 1, users: 0, enabled: 1, f: { ...f } }); return { ok: true, id };
    }
    if (seg[0] === "managers") {
      if (seg[2] === "balance") { const m = mgr(seg[1]); if (m) { m.balance = +m.balance + (+b.Amount || 0); (LEDGER[m.id] = LEDGER[m.id] || []).unshift({ created: dt(Date.now()), delta: +b.Amount || 0, balance_after: m.balance, reason: b.Reason || "", created_by: "admin" }); } return { ok: true }; }
      if (seg[2] === "login") return { ok: true };
      const vals = { username: b.Username, full_name: b.FullName || "", phone: b.Phone || "", parent_id: +b.ParentID || 0, unlimited: b.Unlimited ? 1 : 0, enabled: b.Enabled ? 1 : 0 };
      vals.parent = (mgr(vals.parent_id) || { username: "admin" }).username;
      const m = seg[1] ? mgr(seg[1]) : null;
      if (m) { Object.assign(m, vals); return { ok: true, id: m.id }; }
      const id = Math.max(...MGRS.map(x => x.id)) + 1; MGRS.push({ id, adv_id: 10 + id, balance: 0, debt: 0, grp: "", trial: 0, ...vals }); LEDGER[id] = []; return { ok: true, id };
    }
    if (seg[0] === "nas") {
      if (method === "DELETE") { const i = NAS.findIndex(n => n.id === +seg[1]); if (i >= 0) NAS.splice(i, 1); return { ok: true }; }
      const vals = { name: b.Shortname || "NAS", ip: b.Nasname || "192.0.2.99", type: b.Type || "mikrotik", description: b.Description || "" };
      const n = seg[1] ? NAS.find(x => x.id === +seg[1]) : null;
      if (n) Object.assign(n, vals); else NAS.push({ id: Math.max(0, ...NAS.map(x => x.id)) + 1, ...vals });
      return { ok: true };
    }
    if (seg[0] === "tickets") {
      if (seg[2] === "reply") { const t = TICKETS.find(x => x.id === +seg[1]); if (t) { if (b.Status) t.status = b.Status; if (b.Body) (REPLIES[t.id] = REPLIES[t.id] || []).push({ created_by: "admin", created: dt(Date.now()), body: b.Body }); } return { ok: true }; }
      const id = Math.max(0, ...TICKETS.map(x => x.id)) + 1; TICKETS.unshift({ id, subject: b.Subject || "شكوى", username: b.Username || "", status: "open", created: dt(Date.now()), created_by: "admin", body: b.Body || "" }); REPLIES[id] = []; return { ok: true, id };
    }
    if (path === "cards/generate") {
      const n = Math.min(50, Math.max(1, +b.Count || 10)), series = (b.Series || "S-DEMO-" + (CARDS.length + 1)), p = prof(b.ProfileID), m = mgr(b.ManagerID), codes = [];
      for (let k = 0; k < n; k++) { const code = (b.Prefix || "") + String(5000000000 + CARDS.length * 7919).slice(0, Math.max(6, +b.Length || 10)); codes.push(code);
        CARDS.push({ id: CARDS.length + 1, code, series, kind: b.Kind || "recharge", profile: p.name || "", price: +b.Price || 0, status: "new", manager: m ? m.username : "admin", expires: dt(NOW + 90 * DAY), created: dt(Date.now()) }); }
      return { ok: true, codes, series };
    }
    if (path === "cash") {
      const c = CASH.find(x => x.id === +b.ID);
      switch (b.Action) {
        case "void": if (c) c.status = "void"; return { ok: true };
        case "restore": if (c) c.status = "new"; return { ok: true };
        case "add_manual": { const code = String(9900000000 + CASH.length); CASH.unshift({ id: 501 + CASH.length, code, source: "manual", source_text: "يدوي", wallet: b.Wallet || "", sender_name: "", amount: +b.Amount || 0,
          tx_time: b.Time ? b.Time + ":00" : iso(Date.now()), received: iso(Date.now()), status: "new", used_for: "", used_at: "", used_by: "", ext_ref: b.Ref || "", notes: b.Notes || "", dup_of: "", merged_id: "", suggest: "" }); return { ok: true, code }; }
        case "add_device": DEVICES.push({ id: DEVICES.length + 1, name: b.Name || "موبايل", enabled: 1, paired: 0, idle_min: null, last_seen: "", info: "{}", pair_code: "DEMO" + (DEVICES.length + 1), pair_pin: "1234" }); return { ok: true, code: "DEMO" + DEVICES.length, pin: "1234" };
        case "repair_device": return { ok: true, code: "DEMO-R" + b.ID, pin: "4321" };
        case "toggle_device": { const d = DEVICES.find(x => x.id === +b.ID); if (d) d.enabled = +d.enabled ? 0 : 1; return { ok: true }; }
        case "reparse": return { ok: true, added: 0 };
        default: return { ok: true };
      }
    }
    if (path === "cash/adv") return { ok: true, read: 12, added: 0 };
    if (path === "cafe") {
      const s = CAFE_SERIES.find(x => x.id === +b.ID);
      if (b.Action === "create") { const id = Math.max(0, ...CAFE_SERIES.map(x => x.id)) + 1, cafe = CAFES.find(c => c.id === +b.CafeID) || CAFES[0];
        CAFE_SERIES.push({ id, name: b.Name || "حزمة " + id, notes: b.Notes || "", cafe: cafe.name, cafe_id: cafe.id, profile: prof(b.ProfileID).name || "", qty: +b.Qty || 0, available: +b.Qty || 0, active: 0, ended: 0, online: 0,
          time_val: +b.TimeVal || 0, time_unit: +b.TimeUnit || 2, time_mode: b.TimeMode || "first_login", quota_mb: +b.QuotaMB || 0, price: +b.Price || 0, cost: +b.Cost || 0, mac_max: +b.MacMax || 0, created: dt(Date.now()), enabled: 1 });
        return { ok: true, made: +b.Qty || 0 }; }
      if (b.Action === "toggle_series" && s) { s.enabled = +s.enabled ? 0 : 1; return { ok: true }; }
      if (b.Action === "delete_unused" && s) { const n = s.available; s.qty -= n; s.available = 0; return { ok: true, deleted: n }; }
      return { ok: true };
    }
    if (path === "vpn") {
      const c = CAFES.find(x => x.id === +b.ID);
      if (b.Action === "add") CAFES.push({ id: Math.max(0, ...CAFES.map(x => x.id)) + 1, name: b.Name || "كافيه", notes: b.Notes || "", enabled: 1, connected: false, uptime: "", ip: `10.0.0.${194 + CAFES.length}`,
        username: "cafe-" + (CAFES.length + 1), phone: b.Phone || "", manager: (mgr(b.ManagerID) || {}).username || "", manager_id: +b.ManagerID || 0, online_cards: 0 });
      if (b.Action === "toggle" && c) c.enabled = +c.enabled ? 0 : 1;
      if (b.Action === "delete" && c) CAFES.splice(CAFES.indexOf(c), 1);
      return { ok: true };
    }
    if (path === "speed") {
      if (b.Mode === "off") SPEED = { ...SPEED, state: "off", mode: "off" };
      else SPEED = { state: "on", mode: b.Mode, type: b.Type || "multiply", value: +b.Value || 2, until: b.Mode === "temp" ? dt(Date.now() + (+b.Hours || 1) * HOUR) : "", from: b.From || SPEED.from, to: b.To || SPEED.to };
      return { ok: true };
    }
    if (path === "trash") { (b.IDs || []).forEach(id => { const i = TRASH.findIndex(x => x.id === +id); if (i >= 0) TRASH.splice(i, 1); }); return { ok: true }; }
    if (path === "backups/create") { BACKUPS.unshift({ name: `backup-${dOnly(Date.now())}-${p2(new Date().getHours())}${p2(new Date().getMinutes())}.sql.gz`, date: dt(Date.now()), size: 4.3e6 }); return { ok: true }; }
    if (path === "backups" && method === "DELETE") { (b.Names || []).forEach(n => { const i = BACKUPS.findIndex(x => x.name === n); if (i >= 0) BACKUPS.splice(i, 1); }); return { ok: true }; }
    if (path === "admins" || seg[0] === "admins") {
      const a = seg[1] ? ADMINS.find(x => x.id === +seg[1]) : null, vals = { full_name: b.FullName || "", role: b.Role || "viewer", manager_id: +b.ManagerID || 0, enabled: b.Enabled ? 1 : 0 };
      if (a) Object.assign(a, vals); else ADMINS.push({ id: Math.max(...ADMINS.map(x => x.id)) + 1, username: b.Username || "user", ...vals });
      return { ok: true };
    }
    if (path === "maintenance") return { ok: true, sessions: b.Action === "disconnect_all" ? counts().online : undefined };
    return { ok: true, id: ++nextId };
  }

  // ---------- fetch wrapper
  const apiPath = url => {
    try {
      const u = new URL(url, location.href);
      if (u.protocol !== location.protocol || (u.protocol !== "file:" && u.host !== location.host)) return null; // file:// origins compare as "null", so not by origin
      const m = /\/api\/(.*)$/.exec(u.pathname);
      return m ? { path: m[1].replace(/\/+$/, ""), q: u.searchParams } : null;
    } catch (_) { return null; }
  };
  const json = (data, status = 200) => new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
  async function answer(a, init) {
    await new Promise(r => setTimeout(r, 40 + Math.random() * 80)); // a short wait, so loading states show
    const method = String((init && init.method) || "GET").toUpperCase();
    let body = null;
    if (init && typeof init.body === "string") { try { body = JSON.parse(init.body); } catch (_) {} }
    try {
      const out = method === "GET" ? GET(a.path, a.q) : WRITE(method, a.path, a.q, body);
      if (out && out._status) { const { _status, ...rest } = out; return json(rest, _status); }
      return json(clone(out === undefined ? {} : out));
    } catch (e) {
      console.warn("[demo] " + method + " " + a.path, e);
      return json(method === "GET" ? [] : { ok: true });
    }
  }
  window.fetch = async (input, init) => {
    const a = apiPath(typeof input === "string" ? input : (input && input.url) || String(input));
    if (!a) return realFetch(input, init);
    if (!ON) {
      if (a.path !== "me") return realFetch(input, init);
      try { // a real server answers /api/me with JSON (or 401): keep using it
        const r = await realFetch(input, init);
        if (r.status === 401 || (r.ok && /json/.test(r.headers.get("content-type") || ""))) return r;
      } catch (_) {}
      ON = true; showBadge();
    }
    return answer(a, init);
  };

  // ---------- badge + the few things that leave the page (downloads) in demo mode
  function showBadge() {
    const put = () => {
      if (document.getElementById("demoBadge")) return;
      const b = document.createElement("div");
      b.id = "demoBadge"; b.setAttribute("data-noi18n", "");
      b.textContent = "وضع تجربة — بيانات وهمية";
      b.title = MODE === "live" ? "Demo · Live" : "Demo · Shadow (افتح ?demo&mode=live للوضع التاني)";
      b.style.cssText = "position:fixed;bottom:12px;left:12px;z-index:99999;background:#f59e0b;color:#1f2937;font:700 12px/1.2 Cairo,Tahoma,sans-serif;" +
        "padding:6px 12px;border-radius:999px;box-shadow:0 4px 14px rgba(0,0,0,.18);pointer-events:none;direction:rtl";
      document.body.appendChild(b);
    };
    if (document.body) put(); else document.addEventListener("DOMContentLoaded", put);
  }
  const say = m => { if (typeof window.toast === "function") window.toast(m, true); };
  const realOpen = window.open ? window.open.bind(window) : null;
  window.open = (url, ...rest) => { if (ON && url && apiPath(String(url))) { say("التحميل مش متاح في وضع التجربة"); return null; } return realOpen ? realOpen(url, ...rest) : null; };
  // the subscriber list's CSV / EXCEL buttons navigate to /api/subscribers?format=csv; stop that in demo mode
  document.addEventListener("click", e => { if (ON && e.target.closest && e.target.closest("#tCsv,#tXls")) { e.stopImmediatePropagation(); e.preventDefault(); say("التصدير مش متاح في وضع التجربة"); } }, true);
  if (ON) showBadge();
})();
