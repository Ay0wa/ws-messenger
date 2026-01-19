const $ = (id) => document.getElementById(id);

const state = {
  baseUrl: localStorage.getItem("baseUrl") || "http://localhost:8080",
  token: localStorage.getItem("token") || "",
  user: JSON.parse(localStorage.getItem("user") || "null"),
  ws: null,
  currentChatId: null,
};

const statusEl = $("status");
const chatList = $("chatList");
const messagesEl = $("messages");
const chatTitle = $("chatTitle");
const chatMeta = $("chatMeta");

$("baseUrl").value = state.baseUrl;

function setStatus(text, good = false) {
  statusEl.textContent = text;
  statusEl.style.background = good ? "rgba(46,196,182,0.25)" : "rgba(255,255,255,0.08)";
}

function apiUrl(path) {
  return state.baseUrl.replace(/\/$/, "") + path;
}

function wsUrl(path) {
  const base = state.baseUrl.replace(/\/$/, "");
  if (base.startsWith("https://")) return base.replace("https://", "wss://") + path;
  return base.replace("http://", "ws://") + path;
}

async function request(path, opts = {}) {
  const headers = opts.headers || {};
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  if (opts.body) headers["Content-Type"] = "application/json";
  const res = await fetch(apiUrl(path), { ...opts, headers });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch (_) {}
  if (!res.ok) throw new Error((data && data.error) || res.statusText);
  return data;
}

function saveSession(token, user) {
  state.token = token;
  state.user = user;
  localStorage.setItem("token", token);
  localStorage.setItem("user", JSON.stringify(user));
}

function clearSession() {
  state.token = "";
  state.user = null;
  localStorage.removeItem("token");
  localStorage.removeItem("user");
}

function renderChats(chats) {
  chatList.innerHTML = "";
  if (!chats.length) {
    chatList.textContent = "No chats yet.";
    return;
  }
  chats.forEach((chat) => {
    const card = document.createElement("div");
    card.className = "chat-card" + (chat.id === state.currentChatId ? " active" : "");
    card.innerHTML = `
      <div><strong>${chat.name}</strong></div>
      <div class="meta">Invite: ${chat.invite_code}</div>
    `;
    card.onclick = () => selectChat(chat);
    chatList.appendChild(card);
  });
}

function renderMessages(messages) {
  messagesEl.innerHTML = "";
  if (!messages.length) {
    messagesEl.textContent = "No messages.";
    return;
  }
  messages.slice().reverse().forEach(addMessage);
}

function addMessage(msg) {
  const wrap = document.createElement("div");
  const isSelf = state.user && msg.user_id === state.user.id;
  wrap.className = "message" + (isSelf ? " self" : "");
  const time = new Date(msg.created_at).toLocaleString();
  wrap.innerHTML = `
    <div class="meta">
      <span>${msg.user_id}</span>
      <span>${time}</span>
    </div>
    <div>${msg.content}</div>
  `;
  messagesEl.appendChild(wrap);
  messagesEl.scrollTop = messagesEl.scrollHeight;
}

async function loadChats() {
  if (!state.token) return;
  const chats = await request("/chats");
  renderChats(chats);
}

async function selectChat(chat) {
  state.currentChatId = chat.id;
  chatTitle.textContent = chat.name;
  chatMeta.textContent = `Chat ID: ${chat.id}`;
  renderChats(await request("/chats"));
  const messages = await request(`/chats/${chat.id}/messages?limit=50`);
  renderMessages(messages);
  connectWs(chat.id);
}

function connectWs(chatId) {
  if (state.ws) state.ws.close();
  const tokenParam = state.token ? `?token=${encodeURIComponent(state.token)}` : "";
  state.ws = new WebSocket(wsUrl(`/ws/chats/${chatId}${tokenParam}`));
  setStatus("connecting...");
  state.ws.onopen = () => setStatus("connected", true);
  state.ws.onclose = () => setStatus("disconnected");
  state.ws.onerror = () => setStatus("error");
  state.ws.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.type === "message" && payload.data) {
        addMessage(payload.data);
      }
    } catch (_) {}
  };
}

$("saveBase").onclick = () => {
  state.baseUrl = $("baseUrl").value.trim() || "http://localhost:8080";
  localStorage.setItem("baseUrl", state.baseUrl);
  setStatus("base saved");
};

$("registerBtn").onclick = async () => {
  try {
    const payload = {
      email: $("regEmail").value.trim(),
      password: $("regPassword").value,
      display_name: $("regDisplay").value.trim(),
    };
    const data = await request("/auth/register", { method: "POST", body: JSON.stringify(payload) });
    saveSession(data.token, data.user);
    await loadChats();
    setStatus("registered", true);
  } catch (err) {
    setStatus(err.message);
  }
};

$("loginBtn").onclick = async () => {
  try {
    const payload = {
      email: $("loginEmail").value.trim(),
      password: $("loginPassword").value,
    };
    const data = await request("/auth/login", { method: "POST", body: JSON.stringify(payload) });
    saveSession(data.token, data.user);
    await loadChats();
    setStatus("logged in", true);
  } catch (err) {
    setStatus(err.message);
  }
};

$("createChat").onclick = async () => {
  try {
    const payload = { name: $("chatName").value.trim() };
    if (!payload.name) return;
    await request("/chats", { method: "POST", body: JSON.stringify(payload) });
    $("chatName").value = "";
    await loadChats();
  } catch (err) {
    setStatus(err.message);
  }
};

$("joinChat").onclick = async () => {
  try {
    const code = $("inviteCode").value.trim();
    if (!code) return;
    await request(`/chats/join/${code}`, { method: "POST" });
    $("inviteCode").value = "";
    await loadChats();
  } catch (err) {
    setStatus(err.message);
  }
};

$("disconnectWs").onclick = () => {
  if (state.ws) state.ws.close();
  state.ws = null;
  setStatus("disconnected");
};

$("composer").onsubmit = (e) => {
  e.preventDefault();
  const content = $("messageInput").value.trim();
  if (!content || !state.ws || state.ws.readyState !== WebSocket.OPEN) return;
  state.ws.send(JSON.stringify({ type: "message", content }));
  $("messageInput").value = "";
};

if (state.token) {
  loadChats().catch(() => clearSession());
}
