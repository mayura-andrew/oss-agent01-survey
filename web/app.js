"use strict";

/*
  Survey definition. The keys and option values here must match the Go API
  (main.go) and the database (supabase/schema.sql).
*/
const AGREE = ["Strongly disagree", "Disagree", "Neutral", "Agree", "Strongly agree"];
const VALUE = ["Not valuable", "Slightly valuable", "Moderately valuable", "Very valuable", "Extremely valuable"];

const SECTIONS = [
  {
    id: "profile",
    title: "About you",
    intro: "This helps us compare answers from newcomers and experienced contributors.",
    questions: [
      {
        n: 1, key: "roles", type: "multi", label: "Which best describes your primary role?",
        hint: "Select all that apply.", otherKey: "role_other",
        options: [
          ["student", "Undergraduate or postgraduate student"],
          ["junior", "Junior software engineer or early-career developer"],
          ["experienced", "Experienced software engineer or open-source contributor"],
          ["maintainer", "Open-source project maintainer"],
          ["other", "Other"],
        ],
      },
      {
        n: 2, key: "oss_experience", type: "single",
        label: "How would you rate your experience with open-source software (OSS)?",
        options: [
          ["beginner", "Beginner: I have never contributed, or I am just starting to explore open source."],
          ["intermediate", "Intermediate: I have submitted a few pull requests or fixed minor issues."],
          ["advanced", "Advanced: I regularly contribute to or maintain open-source repositories."],
        ],
      },
      {
        n: 3, key: "barriers", type: "multi", exclusive: "already_contribute",
        label: "What could be the reason for not contributing to open-source software?",
        hint: "Select all that apply.",
        options: [
          ["never_heard", "I have never heard of open-source software."],
          ["no_need", "I never felt the need to contribute to open-source software."],
          ["no_guidance", "I'd like to contribute, but I haven't had proper guidance on how to."],
          ["low_confidence", "I lack confidence in my skills."],
          ["no_benefit", "I feel there is no benefit in contributing to open-source software."],
          ["low_motivation", "I lack motivation."],
          ["already_contribute", "Not applicable: I already contribute."],
        ],
      },
    ],
  },
  {
    id: "pain",
    title: "Getting started with a codebase",
    intro: "How much do you agree with each statement? Choose from 1 (strongly disagree) to 5 (strongly agree).",
    questions: [
      { n: 4, key: "likert_structure", type: "scale", labels: AGREE,
        label: "Understanding the overall structure, and how files and modules connect in an unfamiliar codebase, is one of the hardest parts of getting started." },
      { n: 5, key: "likert_env_setup", type: "scale", labels: AGREE,
        label: "Setting up a project's development environment locally (installing dependencies and build tools) before I can even understand the code discourages me from contributing." },
      { n: 6, key: "likert_docs_outdated", type: "scale", labels: AGREE,
        label: "Text-based documentation (such as README files and wikis) is often outdated, incomplete, or hard to follow for complex projects." },
      { n: 7, key: "likert_tracing_comfortable", type: "scale", labels: AGREE,
        label: "Tracing function calls and execution flows across large codebases using standard text editors is comfortable for me." },
      {
        n: 8, key: "time_to_confidence", type: "single",
        label: "On average, how much time do you spend navigating and understanding an unfamiliar repository before you feel confident enough to make a change or contribution?",
        options: [
          ["lt_1h", "Less than 1 hour"],
          ["1_3h", "1–3 hours"],
          ["4_8h", "4–8 hours"],
          ["gt_8h", "More than 8 hours"],
          ["give_up", "I often give up due to the complexity"],
        ],
      },
    ],
  },
  {
    id: "practice",
    title: "How you explore code today",
    intro: "Tell us what you do now when you open a new project.",
    questions: [
      {
        n: 9, key: "exploration_methods", type: "multi", max: 3,
        label: "When exploring a new codebase for the first time, which methods do you rely on most?",
        hint: "Select up to 3.",
        options: [
          ["readme_docs", "Reading the README.md and project documentation"],
          ["ide_navigation", "Manually jumping through code files in an IDE (for example, VS Code)"],
          ["mental_diagrams", "Drawing mental or paper diagrams of how functions connect"],
          ["tests_debugging", "Running tests and debugging line by line"],
          ["ask_community", "Asking maintainers or community members on Discord, Slack, or GitHub"],
          ["visual_tools", "Using third-party visual architecture tools"],
          ["llm_tools", "Relying on AI tools based on large language models (for example, ChatGPT, Copilot, Claude)"],
        ],
      },
      {
        n: 10, key: "diagram_frequency", type: "single",
        label: "How often do you find architecture diagrams (UML, architecture maps, and so on) in open-source projects that are up to date and helpful?",
        options: [
          ["always", "Always"],
          ["frequently", "Frequently"],
          ["rarely", "Rarely"],
          ["never", "Never, or I almost never see visual diagrams"],
        ],
      },
    ],
  },
  {
    id: "features",
    title: "Ideas for a new tool",
    intro: "How valuable would each feature be in a tool that helps you explore open-source codebases? Choose from 1 (not valuable) to 5 (extremely valuable).",
    questions: [
      { n: 11, key: "feature_zero_install", type: "scale", labels: VALUE,
        label: "Zero installation: open and visualize any repository instantly in your web browser, without installing anything or setting up a backend." },
      { n: 12, key: "feature_visual_map", type: "scale", labels: VALUE,
        label: "Interactive visual code map: a clickable, zoomable graph showing how files, classes, and functions call and import each other." },
      { n: 13, key: "feature_ai_assistant", type: "scale", labels: VALUE,
        label: "Local AI code assistant: an assistant that explains what specific functions or modules do, directly inside the visual map." },
      { n: 14, key: "feature_privacy", type: "scale", labels: VALUE,
        label: "Privacy and offline processing: the tool runs entirely inside your browser, and no code is sent to or stored on external servers." },
      {
        n: 15, key: "likelihood_to_use", type: "single", numeric: true,
        label: "If a free, browser-based visual codebase explorer like this were available, how likely would you be to use it when exploring new open-source projects?",
        options: [
          ["1", "Definitely would not use"],
          ["2", "Unlikely"],
          ["3", "Undecided"],
          ["4", "Likely"],
          ["5", "Definitely would use"],
        ],
      },
      { n: 16, key: "biggest_frustration", type: "text", optional: true,
        label: "What is the single biggest frustration you face when opening an open-source repository for the first time?" },
      { n: 17, key: "how_overcome", type: "text", optional: true,
        label: "If you face challenges, how do you overcome them? What tools or methods do you use?" },
    ],
  },
];

const TEXT_MAX = 2000;
const OTHER_MAX = 200;
const DRAFT_KEY = "oss-agent01-survey-draft-v1";
// Overridden at build time when the page is hosted apart from the API (GitHub Pages).
const API_URL = document.querySelector('meta[name="api-url"]')?.content || "/api/responses";

/* ---------- State ---------- */
let answers = {};
let current = -1; // -1 = intro, 0..3 = sections
let startedAt = null;

function loadDraft() {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return;
    const d = JSON.parse(raw);
    answers = d.answers || {};
    startedAt = d.startedAt || null;
  } catch (_) { /* storage blocked or corrupt: start fresh */ }
}
function saveDraft() {
  try { localStorage.setItem(DRAFT_KEY, JSON.stringify({ answers, startedAt })); } catch (_) {}
}
function clearDraft() {
  try { localStorage.removeItem(DRAFT_KEY); } catch (_) {}
}

/* ---------- DOM helpers ---------- */
const $ = (sel, root = document) => root.querySelector(sel);
function el(tag, attrs = {}, ...kids) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === false || v == null) continue;
    if (k === "class") node.className = v;
    else if (k === "text") node.textContent = v;
    else node.setAttribute(k, v === true ? "" : v);
  }
  for (const kid of kids) if (kid != null) node.append(kid);
  return node;
}

/* ---------- Rendering ---------- */
function renderStepper() {
  const ol = $("#stepper");
  ol.replaceChildren();
  SECTIONS.forEach((s, i) => {
    const li = el("li", {},
      el("span", { class: "num", text: String(i + 1) }),
      el("span", { class: "name", text: s.title }));
    if (i < current) li.classList.add("done");
    if (i === current) li.setAttribute("aria-current", "step");
    ol.append(li);
  });
  ol.hidden = current < 0;
}

function questionLabel(q) {
  const frag = document.createDocumentFragment();
  frag.append(el("span", { class: "q-num", text: `${q.n}.` }), q.label);
  if (q.optional) frag.append(" ", el("span", { class: "optional", text: "(optional)" }));
  return frag;
}

function renderChoice(q) {
  const fs = el("fieldset", { class: "question", id: `q-${q.key}` });
  fs.append(el("legend", {}, questionLabel(q)));
  if (q.hint) fs.append(el("p", { class: "q-hint", id: `hint-${q.key}`, text: q.hint }));
  const list = el("div", { class: "options" });
  const isMulti = q.type === "multi";

  q.options.forEach(([value, text]) => {
    const id = `${q.key}-${value}`;
    const input = el("input", {
      type: isMulti ? "checkbox" : "radio", class: isMulti ? "box" : "dot",
      id, name: q.key, value,
    });
    const current = answers[q.key];
    input.checked = isMulti ? Array.isArray(current) && current.includes(value) : String(current) === value;
    input.addEventListener("change", () => onChoiceChange(q, input));
    const row = el("div", { class: "option" }, input, el("label", { for: id, text }));
    list.append(row);

    if (value === "other" && q.otherKey) {
      const wrap = el("div", { class: "other-input", id: `wrap-${q.otherKey}` });
      const other = el("input", {
        type: "text", class: "text-input", id: q.otherKey, maxlength: OTHER_MAX,
        "aria-label": "Describe your other role",
        placeholder: "Your role",
      });
      other.value = answers[q.otherKey] || "";
      other.addEventListener("input", () => { answers[q.otherKey] = other.value; saveDraft(); });
      wrap.append(other);
      wrap.hidden = !(Array.isArray(answers[q.key]) && answers[q.key].includes("other"));
      list.append(wrap);
    }
  });
  fs.append(list);
  if (q.max) fs.append(el("p", { class: "counter", id: `count-${q.key}`, "aria-live": "polite" }));
  return fs;
}

function renderScale(q) {
  const fs = el("fieldset", { class: "question scale", id: `q-${q.key}` });
  fs.append(el("legend", {}, questionLabel(q)));
  const keys = el("div", { class: "scale-keys" });
  q.labels.forEach((word, i) => {
    const value = String(i + 1);
    const id = `${q.key}-${value}`;
    const input = el("input", { type: "radio", id, name: q.key, value });
    input.checked = String(answers[q.key]) === value;
    input.addEventListener("change", () => {
      answers[q.key] = Number(value);
      updateReadout(q);
      clearFieldError(q.key);
      saveDraft();
    });
    const label = el("label", { for: id },
      el("span", { "aria-hidden": "true", text: value }),
      el("span", { class: "sr-only", text: `${value}, ${word}` }));
    keys.append(el("div", { class: "scale-key" }, input, label));
  });
  fs.append(keys);
  fs.append(el("div", { class: "scale-ends", "aria-hidden": "true" },
    el("span", { text: `1 · ${q.labels[0]}` }),
    el("span", { text: `5 · ${q.labels[4]}` })));
  fs.append(el("p", { class: "scale-readout", id: `readout-${q.key}`, "aria-live": "polite" }));
  return fs;
}

function renderText(q) {
  const wrap = el("div", { class: "question", id: `q-${q.key}` });
  const id = `in-${q.key}`;
  wrap.append(el("label", { class: "q-label", for: id }, questionLabel(q)));
  const ta = el("textarea", { id, maxlength: TEXT_MAX, rows: 4 });
  ta.value = answers[q.key] || "";
  const count = el("p", { class: "counter", id: `count-${q.key}` });
  const update = () => { count.textContent = `${ta.value.length} / ${TEXT_MAX} characters`; };
  ta.addEventListener("input", () => { answers[q.key] = ta.value; update(); saveDraft(); });
  update();
  wrap.append(ta, count);
  return wrap;
}

function renderSection(index) {
  const s = SECTIONS[index];
  const box = $("#sections");
  box.replaceChildren();
  const head = el("div", { class: "section-head" },
    el("h2", { id: "section-title", tabindex: "-1", text: s.title }),
    el("p", { text: s.intro }));
  box.append(head);
  s.questions.forEach((q) => {
    if (q.type === "scale") box.append(renderScale(q));
    else if (q.type === "text") box.append(renderText(q));
    else box.append(renderChoice(q));
  });
  s.questions.forEach((q) => {
    if (q.type === "scale") updateReadout(q);
    if (q.max) updateLimit(q);
  });
}

function updateReadout(q) {
  const out = $(`#readout-${q.key}`);
  if (!out) return;
  const v = answers[q.key];
  out.replaceChildren();
  if (v) out.append("You chose ", el("strong", { text: `${v}, ${q.labels[v - 1]}` }), ".");
}

function updateLimit(q) {
  const chosen = Array.isArray(answers[q.key]) ? answers[q.key] : [];
  const full = chosen.length >= q.max;
  document.querySelectorAll(`input[name="${q.key}"]`).forEach((inp) => {
    inp.disabled = full && !inp.checked;
  });
  const c = $(`#count-${q.key}`);
  if (c) c.textContent = full ? `${chosen.length} of ${q.max} selected. Unselect one to pick another.` : `${chosen.length} of ${q.max} selected.`;
}

/* ---------- Interaction ---------- */
function onChoiceChange(q, input) {
  if (q.type === "multi") {
    let list = Array.isArray(answers[q.key]) ? answers[q.key].slice() : [];
    if (input.checked) list.push(input.value);
    else list = list.filter((v) => v !== input.value);

    // "Not applicable" cannot be combined with other reasons.
    if (q.exclusive && input.checked) {
      list = input.value === q.exclusive ? [q.exclusive] : list.filter((v) => v !== q.exclusive);
      document.querySelectorAll(`input[name="${q.key}"]`).forEach((inp) => { inp.checked = list.includes(inp.value); });
    }
    answers[q.key] = list;

    if (q.otherKey) {
      const wrap = $(`#wrap-${q.otherKey}`);
      const on = list.includes("other");
      wrap.hidden = !on;
      if (on && input.value === "other") $(`#${q.otherKey}`).focus();
      if (!on) { answers[q.otherKey] = ""; $(`#${q.otherKey}`).value = ""; }
    }
    if (q.max) updateLimit(q);
  } else {
    answers[q.key] = q.numeric ? Number(input.value) : input.value;
  }
  clearFieldError(q.key);
  saveDraft();
}

/* ---------- Validation ---------- */
function validateSection(index) {
  const errors = [];
  for (const q of SECTIONS[index].questions) {
    if (q.optional) continue;
    const v = answers[q.key];
    let msg = null;
    if (q.type === "multi") {
      if (!Array.isArray(v) || v.length === 0) msg = "Select at least one answer.";
      else if (q.max && v.length > q.max) msg = `Select no more than ${q.max}.`;
      else if (q.otherKey && v.includes("other") && !(answers[q.otherKey] || "").trim()) msg = "Describe your other role, or unselect Other.";
    } else if (q.type === "scale" || q.type === "single") {
      if (v == null || v === "") msg = q.type === "scale" ? "Choose a number from 1 to 5." : "Choose one answer.";
    }
    if (msg) errors.push({ q, msg });
  }
  return errors;
}

function clearFieldError(key) {
  const box = $(`#q-${key}`);
  if (!box) return;
  box.classList.remove("has-error");
  const e = $(`#err-${key}`);
  if (e) e.remove();
  box.removeAttribute("aria-describedby");
  box.removeAttribute("aria-invalid");
}

function showErrors(errors) {
  const summary = $("#error-summary");
  errors.forEach(({ q, msg }) => {
    clearFieldError(q.key);
    const box = $(`#q-${q.key}`);
    box.classList.add("has-error");
    box.setAttribute("aria-invalid", "true");
    const p = el("p", { class: "field-error", id: `err-${q.key}` },
      el("span", { class: "err-icon", "aria-hidden": "true", text: "!" }), msg);
    box.append(p);
    box.setAttribute("aria-describedby", `err-${q.key}`);
  });
  const list = el("ul");
  errors.forEach(({ q, msg }) => {
    const a = el("a", { href: `#q-${q.key}`, text: `Question ${q.n}: ${msg}` });
    a.addEventListener("click", (ev) => {
      ev.preventDefault();
      const target = $(`#q-${q.key}`);
      target.scrollIntoView({ block: "center" });
      const first = target.querySelector("input, textarea");
      if (first) first.focus();
    });
    list.append(el("li", {}, a));
  });
  const n = errors.length;
  summary.replaceChildren(
    el("h2", { text: n === 1 ? "1 question needs an answer" : `${n} questions need an answer` }),
    list);
  summary.hidden = false;
  summary.focus();
}

function hideSummary() {
  const s = $("#error-summary");
  s.hidden = true;
  s.replaceChildren();
}

/* ---------- Navigation ---------- */
function goTo(index) {
  current = index;
  hideSummary();
  $("#submit-error").hidden = true;
  renderStepper();
  renderSection(index);
  $("#back").hidden = false;
  const last = index === SECTIONS.length - 1;
  $("#next").textContent = last ? "Submit answers" : `Next: ${SECTIONS[index + 1].title}`;
  window.scrollTo({ top: $(".masthead").offsetHeight - 16, behavior: "auto" });
  $("#section-title").focus({ preventScroll: true });
}

function buildPayload() {
  const p = {
    consent: true,
    roles: answers.roles || [],
    role_other: (answers.role_other || "").trim(),
    oss_experience: answers.oss_experience || "",
    barriers: answers.barriers || [],
    exploration_methods: answers.exploration_methods || [],
    time_to_confidence: answers.time_to_confidence || "",
    diagram_frequency: answers.diagram_frequency || "",
    biggest_frustration: (answers.biggest_frustration || "").trim(),
    how_overcome: (answers.how_overcome || "").trim(),
    completion_seconds: startedAt ? Math.round((Date.now() - startedAt) / 1000) : 0,
    website: $("#website").value,
  };
  ["likert_structure", "likert_env_setup", "likert_docs_outdated", "likert_tracing_comfortable",
   "feature_zero_install", "feature_visual_map", "feature_ai_assistant", "feature_privacy",
   "likelihood_to_use"].forEach((k) => { p[k] = Number(answers[k]) || 0; });
  if (!p.roles.includes("other")) p.role_other = "";
  return p;
}

async function submit() {
  const btn = $("#next");
  const err = $("#submit-error");
  err.hidden = true;
  btn.setAttribute("aria-busy", "true");
  btn.disabled = true;
  btn.textContent = "Submitting…";
  try {
    const res = await fetch(API_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPayload()),
    });
    if (res.status === 201) {
      clearDraft();
      $("#survey").hidden = true;
      $("#stepper").hidden = true;
      const t = $("#thanks");
      t.hidden = false;
      t.focus();
      return;
    }
    if (res.status === 429) throw new Error("Too many submissions from this network. Wait a few minutes, then submit again.");
    if (res.status === 422) {
      const data = await res.json().catch(() => ({}));
      const fields = data && data.errors ? Object.keys(data.errors).join(", ") : "";
      throw new Error(`Some answers weren't accepted${fields ? ` (${fields})` : ""}. Check your answers, then submit again.`);
    }
    throw new Error("Your answers weren't saved because the server didn't respond correctly. Your answers are kept on this page; submit again in a moment.");
  } catch (e) {
    const offline = e instanceof TypeError;
    err.textContent = offline
      ? "Your answers weren't saved because there is no connection. Your answers are kept on this page; check your connection and submit again."
      : e.message;
    err.hidden = false;
    btn.disabled = false;
    btn.removeAttribute("aria-busy");
    btn.textContent = "Submit answers";
  }
}

function onNext() {
  const errors = validateSection(current);
  if (errors.length) { showErrors(errors); return; }
  if (current < SECTIONS.length - 1) goTo(current + 1);
  else submit();
}

function onBack() {
  if (current > 0) goTo(current - 1);
  else {
    current = -1;
    $("#survey").hidden = true;
    $("#intro").hidden = false;
    renderStepper();
    $("#intro-title").focus();
  }
}

function start() {
  const box = $("#consent");
  const e = $("#consent-error");
  if (!box.checked) {
    e.hidden = false;
    box.setAttribute("aria-invalid", "true");
    box.setAttribute("aria-describedby", "consent-error");
    box.focus();
    return;
  }
  e.hidden = true;
  box.removeAttribute("aria-invalid");
  if (!startedAt) startedAt = Date.now();
  saveDraft();
  $("#intro").hidden = true;
  $("#survey").hidden = false;
  goTo(0);
}

document.addEventListener("DOMContentLoaded", () => {
  loadDraft();
  $("#intro-title").setAttribute("tabindex", "-1");
  $("#start").addEventListener("click", start);
  $("#next").addEventListener("click", onNext);
  $("#back").addEventListener("click", onBack);
  $("#consent").addEventListener("change", (ev) => {
    if (ev.target.checked) $("#consent-error").hidden = true;
  });
  // Enter key inside a text field should not submit the form.
  $("#survey").addEventListener("submit", (ev) => ev.preventDefault());
});
