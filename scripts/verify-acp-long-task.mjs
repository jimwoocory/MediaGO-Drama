import assert from "node:assert/strict";
import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { resolve, join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

// Real server + bundled Codex ACP, scripted loopback model responses only.
// All configuration, generated documents and logs live in a fresh test workspace.
const args = process.argv.slice(2);
function option(name, fallback) {
  const i = args.indexOf(name);
  return i < 0 ? fallback : args[i + 1];
}
const binary = option("--server");
const agentDir = option("--agent-dir");
const scenario = option("--scenario", "success");
assert(binary && existsSync(binary), "--server must name an existing test server binary");
assert(agentDir && existsSync(agentDir), "--agent-dir must name the bundled agent directory");
assert(["success", "compaction", "429", "cancel"].includes(scenario), "unknown scenario");
const output = resolve(import.meta.dirname, "../../MediaGo-Builds/verification/jw-acp-longtask-validation");
mkdirSync(output, { recursive: true });
const workspace = mkdtempSync(join(output, scenario + "-"));
const codexHome = join(workspace, "isolated-codex-home");
mkdirSync(codexHome);
const projectDir = join(workspace, "fixture-project");
mkdirSync(projectDir);
const model = "jw-long-task-fixture";
const chapters = 54;
const report = { scenario, workspace, model, kind: "scripted-loopback-real-codex-acp", requests: [], compactions: 0 };
let sidecar, apiOrigin, agentBase, sessionID, lastLog = "", nextStep = 0, retryResponses = 0;
let compactionPending = false, cancelRequested = false;
const psString = s => "'" + s.replaceAll("'", "''") + "'";
const header = "---\ntitle: JW long-task fixture\ncategory: screenplay\n---\n\n";
const chapter = i => "## Chapter " + i + "\nJW_BATCH_" + i + "\n\n";
const upstream = createServer(async (req, res) => {
  res.setHeader("Content-Type", "application/json");
  try {
    if (req.url === "/v1/models") {
      res.end(JSON.stringify({ data: [{ id: model, context_length: 32768 }] }));
      return;
    }
    assert.equal(req.url, "/v1/chat/completions", "unexpected model endpoint");
    let raw = "";
    for await (const chunk of req) raw += chunk;
    const body = JSON.parse(raw);
    const functions = (body.tools || []).map(t => t.function).filter(Boolean);
    const compact = functions.length === 0;
    report.requests.push({ number: report.requests.length + 1, model: body.model, compact, step: nextStep });
    assert.equal(body.model, model, "request escaped selected fixture model");
    assert(report.requests.length <= 100, "fixture request watchdog exceeded");
    let message, promptTokens = 1500;
    if (compact) {
      report.compactions++;
      compactionPending = false;
      message = { role: "assistant", content: "Continue the authorized fixture task. script.md is the authoritative progress record. Preserve its completed chapters, append only the remaining batches, and do not generate media." };
    } else if (scenario === "429" && nextStep >= 14) {
      retryResponses++;
      res.statusCode = 429;
      res.end(JSON.stringify({ error: { code: "rate_limit_exceeded", type: "rate_limit_error", message: "JW local fixture rate limit" } }));
      return;
    } else {
      const step = nextStep++;
      const call = (name, input) => ({
        role: "assistant", content: null,
        tool_calls: [{ id: "jw_batch_call_" + step, type: "function", function: { name, arguments: JSON.stringify(input) } }],
      });
      if (step === 0) {
        const skillTool = functions.find(t => t.name.endsWith("__load_skill"));
        assert(skillTool, "load_skill missing from real ACP tool catalog");
        message = call(skillTool.name, { name: "screenplay-writer" });
      } else if (step === 1) {
        message = call("shell_command", {
          command: "[System.IO.File]::WriteAllText('script.md', " + psString(header) + ", [System.Text.UTF8Encoding]::new($false))",
          login: false,
        });
      } else if (step <= chapters + 1) {
        message = call("shell_command", {
          command: "[System.IO.File]::AppendAllText('script.md', " + psString(chapter(step - 1)) + ", [System.Text.UTF8Encoding]::new($false))",
          login: false,
        });
      } else {
        message = { role: "assistant", content: "JW_LONG_TASK_COMPLETE" };
      }
      if (scenario === "compaction" && [19, 37, 55].includes(step)) compactionPending = true;
      if (compactionPending) promptTokens = 30000;
      if (scenario === "cancel" && step === 14) {
        // Hold an in-flight model response until the test requests user cancellation.
        cancelRequested = true;
        await delay(1500);
      }
    }
    if (!res.destroyed) res.end(JSON.stringify({
      id: "jw-response-" + report.requests.length, object: "chat.completion", model: body.model,
      choices: [{ index: 0, message, finish_reason: message.tool_calls ? "tool_calls" : "stop" }],
      usage: { prompt_tokens: promptTokens, completion_tokens: 30, total_tokens: promptTokens + 30 },
    }));
  } catch (err) {
    report.fixtureError = String(err);
    res.statusCode = 500;
    if (!res.destroyed) res.end(JSON.stringify({ error: { message: String(err) } }));
  }
});
async function api(path, method = "GET", body) {
  const response = await fetch(apiOrigin + "/api/v1" + path, {
    method, headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(10000),
  });
  const data = await response.json();
  if (!response.ok || data.success === false) throw new Error(path + ": " + JSON.stringify(data));
  return data.data;
}
try {
  await new Promise(r => upstream.listen(0, "127.0.0.1", r));
  const probe = createServer();
  await new Promise(r => probe.listen(0, "127.0.0.1", r));
  const port = probe.address().port;
  await new Promise(r => probe.close(r));
  apiOrigin = "http://127.0.0.1:" + port;
  const configPath = join(workspace, "server.yaml");
  writeFileSync(configPath, JSON.stringify({
    host: "127.0.0.1", port, workspace_dir: workspace, log_level: "info",
    agent: { id: "codex", bin_dir: resolve(agentDir) },
    prompt: { instruction_delivery: "native", max_section_chars: 12000 },
  }));
  const env = { ...process.env };
  for (const key of Object.keys(env)) {
    if (/^(MEDIAGO_|OPENAI_|CODEX_|ANTHROPIC_|GEMINI_|GOOGLE_API_KEY)/i.test(key)) delete env[key];
  }
  env.CODEX_HOME = codexHome;
  env.CODEX_CONFIG = "{}";
  sidecar = spawn(resolve(binary), ["--config", configPath], { env, windowsHide: true, stdio: ["ignore", "pipe", "pipe"] });
  for (const stream of [sidecar.stdout, sidecar.stderr]) stream.on("data", data => { lastLog = (lastLog + data).slice(-16000); });
  for (let i = 0; ; i++) {
    try { await api("/projects"); break; }
    catch (err) { if (i >= 80 || sidecar.exitCode !== null) throw err; await delay(250); }
  }
  const project = await api("/projects", "POST", { name: "JW long-task regression", projectDir });
  const endpoint = "http://127.0.0.1:" + upstream.address().port + "/v1";
  await api("/settings/codex-relay", "PUT", {
    enabled: true, activeProfileId: "longtask",
    profiles: [{ id: "longtask", name: "Local fixture", baseURL: endpoint, model, protocol: "chatCompletions", enabled: true }],
  });
  await api("/settings/codex-relay/profiles/longtask/api-key", "PUT", { apiKey: "local-fixture-no-real-credential" });
  agentBase = "/projects/" + project.id + "/agent";
  const session = await api(agentBase + "/sessions", "POST", { newSession: true });
  sessionID = session.sessionId;
  const sessionBase = agentBase + "/sessions/" + sessionID;
  await api(sessionBase + "/messages", "POST", {
    prompt: "In this isolated regression project, load screenplay-writer and write script.md in 54 short numbered batches. Preserve each completed batch. No media generation. Finish with JW_LONG_TASK_COMPLETE.",
    model: { configId: "model", source: "configOption", value: "api-longtask:" + model },
  });
  let status, sentCancel = false;
  const approved = new Set();
  for (let i = 0; i < 840; i++) {
    await delay(500);
    if (cancelRequested && !sentCancel) {
      await api(sessionBase + "/cancel", "POST", {});
      sentCancel = true;
    }
    status = await api(sessionBase + "/status");
    for (const permission of status.pendingPermissions || []) {
      if (approved.has(permission.requestId)) continue;
      // Approve only this fixture's generated file operations, one request at a
      // time. Keep the application's normal approval policy unchanged.
      const callID = permission.toolCall?.id || "";
      assert(/^jw_batch_call_\d+$/.test(callID), "unexpected permission tool");
      const step = Number(callID.slice("jw_batch_call_".length));
      assert(step >= 1 && step <= chapters + 1 && step < nextStep, "unexpected permission step");
      const events = readFileSync(join(projectDir, "agent-sessions", sessionID, "acp-events.jsonl"), "utf8");
      const request = events.trim().split("\n").map(line => JSON.parse(line)).filter(e => e.source === "process.stdout")
        .map(e => { try { return JSON.parse(e.raw); } catch { return null; } })
        .findLast(e => e?.method === "session/request_permission" && e.params?.toolCall?.toolCallId === callID);
      assert(request, "missing raw approval evidence");
      assert.equal(resolve(request.params.toolCall.rawInput.cwd), resolve(projectDir, "work"), "permission escaped fixture workspace");
      assert(request.params.toolCall.rawInput.command.includes("script.md"), "unexpected permission command");
      const allowOnce = permission.options.find(o => o.kind === "allow_once");
      assert(allowOnce, "single-use approval option missing");
      await api(sessionBase + "/permission-requests/" + permission.requestId + "/decision", "POST", { optionId: allowOnce.optionId });
      approved.add(permission.requestId);
    }
    if (!status.running) break;
    if (i % 40 === 0) console.log(JSON.stringify({ scenario, requests: report.requests.length, nextStep, compactions: report.compactions }));
  }
  report.status = status;
  report.chat = await api(sessionBase + "/chat");
  const hasFinalSuccess = report.chat.messages.some(m =>
    m.role === "assistant" && m.kind === "message" && m.content?.trim() === "JW_LONG_TASK_COMPLETE");
  assert(!report.fixtureError, report.fixtureError);
  assert(status && !status.running, "run did not terminate before test timeout");
  const docPath = join(projectDir, "work", "script.md");
  assert(existsSync(docPath), "real tools did not create the fixture document");
  const document = readFileSync(docPath, "utf8");
  const count = [...document.matchAll(/JW_BATCH_(\d+)/g)].length;
  const expectedCount = scenario === "429" ? 12 : scenario === "cancel" ? count : chapters;
  assert(count > 0, "no batches were persisted");
  // The workspace legitimately adds frontmatter IDs/versions and section IDs.
  // Compare every body byte after removing only those generated annotations.
  assert(document.includes("title: JW long-task fixture\n"), "document title was lost");
  assert(document.includes("category: screenplay\n"), "document category was lost");
  const body = document.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n/, "")
    .replace(/^<!-- section-id: [A-Za-z0-9_-]+ -->\r?\n/gm, "").trim();
  const expectedBody = Array.from({ length: expectedCount }, (_, i) => chapter(i + 1)).join("").trim();
  assert.equal(body, expectedBody, "missing, duplicated or out-of-order batches");
  const documents = await api("/projects/" + project.id + "/workspace/documents");
  assert.equal(documents.documents.length, 1, "background sync split the original document");
  if (scenario === "429") {
    assert.equal(status.lastStatus, "failed", "429 must not become successful completion");
    assert(retryResponses > 0 && retryResponses <= 6, "unexpected retry count");
    assert(!hasFinalSuccess, "false final success after 429");
  } else if (scenario === "cancel") {
    assert(sentCancel, "cancellation was not requested");
    assert.notEqual(status.lastStatus, "completed", "cancelled task was reported completed");
    assert(count < chapters, "cancel request did not interrupt the task");
  } else {
    assert.equal(status.lastStatus, "completed", "long task did not complete");
    assert(hasFinalSuccess, "final answer missing");
    if (scenario === "compaction") assert(report.compactions >= 3, "native compaction path not exercised");
  }
  const requestsAfterStop = report.requests.length;
  await delay(1500);
  assert.equal(report.requests.length, requestsAfterStop, "upstream calls continued after terminal state");
  assert.equal(readFileSync(docPath, "utf8"), document, "document changed after terminal state");
  report.verified = { batches: count, exactDocument: true, compactions: report.compactions, retryResponses, singleUseApprovals: approved.size, stopped: true, paidCalls: false };
  console.log("VERIFIED", JSON.stringify(report.verified));
} catch (err) {
  report.error = String(err);
  report.serverLogTail = lastLog;
  console.error(String(err));
  process.exitCode = 1;
} finally {
  writeFileSync(join(workspace, "report.json"), JSON.stringify(report, null, 2));
  console.log("REPORT", join(workspace, "report.json"));
  if (sidecar && sidecar.exitCode === null) {
    if (process.platform === "win32") {
      const kill = spawn("taskkill", ["/PID", String(sidecar.pid), "/T", "/F"], { windowsHide: true, stdio: "ignore" });
      await new Promise(r => { kill.once("exit", r); kill.once("error", r); });
    } else sidecar.kill();
  }
  upstream.closeAllConnections();
  upstream.close();
}
