import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { resolve, join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

const repo = resolve(import.meta.dirname, "..");
const output = resolve(repo, "../MediaGo-Builds/verification/jw-provider-validation");
mkdirSync(output, { recursive: true });
const workspace = mkdtempSync(join(output, "workspace-"));
const codexHome = join(workspace, ".codex");
mkdirSync(codexHome);
const report = { kind: "scripted-upstream-real-codex-acp", workspace, requests: [], results: [] };
const fixtureModel = process.env.JW_SMOKE_MODEL || "deepseek/DeepSeek-V3.2:fixture";
const unified = process.env.JW_SMOKE_UNIFIED === "1";
const failure = process.env.JW_SMOKE_FAILURE || "";
const reasoning = process.env.JW_SMOKE_REASONING || "";
const planSync = process.env.JW_SMOKE_PLAN_SYNC === "1";
if (["loop", "compaction"].includes(failure)) {
  throw new Error("Legacy count-based stop fixtures were retired. Use scripts/verify-acp-long-task.mjs for finite long-task and native-compaction regression checks.");
}
const contextWindow = Number(process.env.JW_SMOKE_CONTEXT || 0);
const providerID = unified ? "gateway-aihubmix" : "api-fixture";
let sidecar;
let lastLog = "";
const upstream = createServer(async (req, res) => {
  res.setHeader("Content-Type", "application/json");
  if (req.url === "/v1/models") { res.end(JSON.stringify({ data: [{ id: fixtureModel, context_length:contextWindow }] })); return; }
  let raw = ""; for await (const chunk of req) raw += chunk;
  const body = JSON.parse(raw || "{}");
  if (process.env.JW_SMOKE_RESPONSES === "1") {
    report.requests.push({ model: body.model, reasoningEffort: body.reasoning?.effort });
    report.rawTools = body.tools;
    console.log("RAW_TOOLS", JSON.stringify((body.tools || []).map(t=>({type:t.type,name:t.name,tools:t.tools?.map(child=>child.name)}))));
    const response = {id:"resp_fixture",object:"response",status:"completed",model:body.model,output:[{type:"message",id:"msg_fixture",status:"completed",role:"assistant",content:[{type:"output_text",text:"JW_PROVIDER_TEXT_OK",annotations:[]}]}],usage:{input_tokens:100,output_tokens:10,total_tokens:110}};
    res.setHeader("Content-Type","text/event-stream");
    res.end("event: response.completed\ndata: "+JSON.stringify({type:"response.completed",response})+"\n\n");
    return;
  }
  const tools = (body.tools || []).map(t => t.function);
  report.requests.push({ model: body.model, tools, reasoningEffort: body.reasoning_effort });
  const outputs = (body.messages || []).filter(m => m.role === "tool");
  report.results = outputs;
  console.log(JSON.stringify({ request: report.requests.length, model: body.model, toolCount: tools.length, outputCount:outputs.length }));
  const step = outputs.length;
  if (failure === "429" && step >= 2) {
    res.statusCode = 429;
    res.end(JSON.stringify({error:{code:"rate_limit_exceeded",type:"rate_limit_error",message:"JW fixture rate limit"}}));
    return;
  }
  const call = (name, args) => ({role:"assistant",content:null,tool_calls:[{id:"call_fixture_"+step,type:"function",function:{name,arguments:JSON.stringify(args)}}]});
  let message;
  if(planSync) {
    if(step===0) message=call("update_plan",{plan:Array.from({length:9},(_,index)=>({step:"Fixture script step "+(index+1),status:index<4?"completed":index===4?"in_progress":"pending"}))});
    else if(step===1) message=call("shell_command",{command:"1..7 | ForEach-Object { '## Scene ' + $_ } | Set-Content -LiteralPath 'jw-plan-fixture.md'",login:false});
    else if(step===2) message=call("shell_command",{command:"if (@(Select-String -LiteralPath 'jw-plan-fixture.md' -Pattern '^## Scene [1-7]$').Count -ne 7) { throw 'incomplete fixture' }; Write-Output JW_SEVEN_SCENES_VERIFIED",login:false});
    else message={role:"assistant",content:"JW_PLAN_SYNC_DONE"};
  }
  else if(failure==="compaction") message=tools.length===0 ? {role:"assistant",content:"JW fixture summary: continue the read-only validation task."} : call("shell_command",{command:"Write-Output JW_COMPACTION_"+report.requests.length,login:false});
  else if(failure==="loop") message=call("shell_command",{command:"Write-Output JW_READ_ONLY_"+report.requests.length,login:false});
  else if(step===0) message=call("shell_command",{command:"Get-Location",login:false});
  else if(step===1) message=call("shell_command",{command:"Get-Content -LiteralPath '"+join(codexHome,"skills/jw-fixture/SKILL.md").replaceAll("'","''")+"'",login:false});
  else if(step===2) message=call(tools.find(t=>t.name.endsWith("__get_project_config"))?.name || "missing_mcp",{});
  else if(step===3) message=call(tools.find(t=>t.name.endsWith("__generate_media"))?.name || "missing_generation_mcp",{kind:"image",prompt:"JW validation: this unconfirmed request must be rejected before submission."});
  else message={role:"assistant",content:"JW_PROVIDER_TOOLS_OK"};
  res.end(JSON.stringify({ id: "jw-fixture-"+report.requests.length, object: "chat.completion", model: body.model, choices: [{ index: 0, message, finish_reason: message.tool_calls ? "tool_calls" : "stop" }], usage: { prompt_tokens: failure==="compaction"?30000:100, completion_tokens: 10, total_tokens: failure==="compaction"?30010:110 } }));
});
await new Promise(r => upstream.listen(0, "127.0.0.1", r));
const upstreamPort = upstream.address().port;
const portProbe = createServer();
await new Promise(r => portProbe.listen(0, "127.0.0.1", r));
const port = portProbe.address().port;
await new Promise(r=>portProbe.close(r));
const apiOrigin = "http://127.0.0.1:"+port;
async function api(path, method="GET", body) {
  const response = await fetch(apiOrigin+"/api/v1"+path, { method, headers: { "Content-Type":"application/json" }, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await response.json();
  if (!response.ok || data.success === false) throw new Error(path+": "+JSON.stringify(data));
  return data.data;
}
try {
  const configPath = join(workspace,"server.yaml");
  mkdirSync(join(codexHome,"skills/jw-fixture"),{recursive:true});
  writeFileSync(join(codexHome,"skills/jw-fixture/SKILL.md"),"---\nname: jw-fixture\ndescription: Read-only JW provider validation fixture\n---\nJW_SKILL_READ_OK\n");
  writeFileSync(configPath, JSON.stringify({ host:"127.0.0.1", port, workspace_dir:workspace, log_level:"debug", agent:{id:"codex",bin_dir:process.env.JW_SMOKE_AGENT_DIR || join(repo,"apps/workspace/electron/resources/agents")}, prompt:{instruction_delivery:"native",max_section_chars:12000} }));
  const env = { ...process.env, CODEX_HOME:codexHome, CODEX_CONFIG:"{}" };
  for (const name of Object.keys(env)) if (name.startsWith("MEDIAGO_")) delete env[name];
  sidecar = spawn(process.env.JW_SMOKE_SERVER || join(output,"mediago-server.exe"),["--config",configPath],{env,windowsHide:true,stdio:["ignore","pipe","pipe"]});
  for (const stream of [sidecar.stdout,sidecar.stderr]) stream.on("data",chunk=>{lastLog=(lastLog+chunk).slice(-12000);});
  for(let i=0;i<80;i++){try{await api("/projects");break;}catch(err){if(i===79)throw err;await delay(250);}}
  const projectDir=join(workspace,"provider-test-project");
  mkdirSync(projectDir);
  const project=await api("/projects","POST",{name:"JW Provider Tools Validation",projectDir});
  console.log("PROJECT",JSON.stringify(project));
  if (unified) {
    await api("/settings/aihubmix","PUT",{baseURL:"http://127.0.0.1:"+upstreamPort+"/v1"});
    await api("/settings/api-keys/aihubmix","PUT",{apiKey:"fixture-key"});
  } else {
    await api("/settings/codex-relay","PUT",{enabled:true,activeProfileId:"fixture",profiles:[{id:"fixture",name:"Fixture Gateway",baseURL:"http://127.0.0.1:"+upstreamPort+"/v1",model:fixtureModel,protocol:process.env.JW_SMOKE_RESPONSES === "1" ? "responses" : "chatCompletions",enabled:true}]});
    await api("/settings/codex-relay/profiles/fixture/api-key","PUT",{apiKey:"fixture-key"});
  }
  const base="/projects/"+project.id+"/agent";
  report.runtimeConfig=await api(base+"/runtime-config");
  if(!report.runtimeConfig.model.options.some(option=>option.providerId===providerID && option.modelId===fixtureModel)) throw new Error("runtime picker did not expose the selected API provider");
  if(unified && report.runtimeConfig.model.options.filter(option=>option.providerId?.startsWith("gateway-")).length!==1) throw new Error("unified provider was duplicated");
  const session=await api(base+"/sessions","POST",{newSession:true});
  await api(base+"/sessions/"+session.sessionId+"/messages","POST",{prompt:planSync ? "Test plan reporting in this isolated test project. Create jw-plan-fixture.md with seven scene headings and verify all seven headings. Reply JW_PLAN_SYNC_DONE." : "Perform a read-only tool validation in this test project, then reply JW_PROVIDER_TEXT_OK.",model:{configId:"model",source:"configOption",value:providerID+":"+fixtureModel},reasoning:{configId:"reasoning_effort",source:reasoning ? "providerReasoning" : "configOption",value:reasoning ? "provider:"+reasoning : "high"}});
  let status;
  const fixtureDecisions=new Set();
  for(let i=0;i<240;i++){
    await delay(500);
    status=await api(base+"/sessions/"+session.sessionId+"/status");
    if(planSync) for(const permission of status.pendingPermissions || []) {
      if(fixtureDecisions.has(permission.requestId)) continue;
      // Only the two fixed shell calls authored above are eligible, in this
      // script's newly created project. Never approve a session-wide policy.
      if(!["call_fixture_1","call_fixture_2"].includes(permission.toolCall?.id)) throw new Error("unexpected fixture permission");
      const option=permission.options.find(option=>option.kind==="allow_once");
      if(!option) throw new Error("fixture has no single-use permission option");
      await api(base+"/sessions/"+session.sessionId+"/permission-requests/"+permission.requestId+"/decision","POST",{optionId:option.optionId});
      fixtureDecisions.add(permission.requestId);
    }
    if(!status.running)break;
  }
  report.status=status;
  report.chat=await api(base+"/sessions/"+session.sessionId+"/chat");
  if (reasoning && (!report.requests.length || report.requests.some(request => request.reasoningEffort !== (reasoning === "default" ? undefined : reasoning)))) throw new Error("selected reasoning effort did not reach the upstream");
  if (JSON.stringify(report.chat).includes("fallback metadata")) throw new Error("model catalog was not applied");
  if (lastLog.includes("acp reasoning config ignored")) throw new Error("stale OAuth reasoning was submitted");
  console.log("STATUS",JSON.stringify(status));
  if (failure) {
    if(status?.lastStatus!=="failed" || status.running) throw new Error("failure was not reported as failed");
    if(failure==="compaction" && !status.lastMessage?.includes("重复压缩")) throw new Error("native compaction guard did not activate");
    if(report.requests.length > (failure === "429" ? 8 : 12)) throw new Error("run exceeded bounded request budget");
    const count=report.requests.length;
    await delay(1500);
    if(report.requests.length!==count) throw new Error("upstream calls continued after failure");
    report.verified={failure,failedStatus:true,requests:count,stoppedUpstream:true,paidGeneration:false};
  } else if(status?.lastStatus!=="completed") throw new Error("run did not complete");
  if(planSync) {
    const plans=report.chat.messages.filter(message=>message.kind==="plan");
    const plan=plans.at(-1);
    const entries=plan?.metadata?.planEntries || [];
    if(plans.length!==1 || entries.length!==9 || entries.filter(entry=>entry.status==="completed").length!==4) throw new Error("native incomplete plan was lost or falsely completed");
    if(!plan.metadata.planToolStates) throw new Error("plan execution checkpoint was not persisted");
    const laterTools=report.chat.messages.filter(message=>message.kind==="tool" && message.turnId===plan.turnId && message.metadata?.status==="completed" && plan.metadata.planToolStates[message.metadata.toolCallId]!=="completed");
    if(laterTools.length<2 || !report.results.some(result=>String(result.content).includes("JW_SEVEN_SCENES_VERIFIED"))) throw new Error("later writing and validation results did not survive chat reload");
    if(report.requests.length!==4) throw new Error("unexpected extra model request");
    report.verified={nativePlan:true,checkpointPersisted:true,laterCompletedTools:laterTools.length,confirmedSteps:4,totalSteps:9,sevenSceneFixture:true,runCompleted:true,noExtraModelRequest:true,paidGeneration:false};
  }
  if(!failure && !planSync && process.env.JW_SMOKE_RESPONSES !== "1") {
    const outputs=report.results.map(o=>String(o.content));
    if(outputs.length!==4 || !outputs[0].includes("Exit code: 0") || !outputs[1].includes("JW_SKILL_READ_OK") || !outputs[2].includes('"status":"ok"') || !/confirmation/i.test(outputs[3])) throw new Error("tool output assertions failed");
    report.verified={terminal:true,skillFile:true,documentMCP:true,generationMCPConfirmationGuard:true,paidGeneration:false};
  }
} catch(err) {
  report.error=String(err);
  console.error(String(err));console.error(lastLog);
  process.exitCode=1;
} finally {
  writeFileSync(join(workspace,"report.json"),JSON.stringify(report,null,2));
  console.log("REPORT",join(workspace,"report.json"));
  if(sidecar)sidecar.kill();
  upstream.closeAllConnections();upstream.close();
}
