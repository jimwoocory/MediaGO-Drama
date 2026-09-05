import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, writeFileSync, readFileSync } from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

// Mock aggregators only by default. --codex-generate deliberately spends ONE
// subscription image request and must only be used with the user's approval.
const root = path.resolve(process.env.JW_MEDIA_TEST_OUTPUT || 'D:/openai/MediaGo-Builds/verification/unified-media');
mkdirSync(root, { recursive: true });
const output = mkdtempSync(path.join(root, 'qa-'));
const oldBuild = 'D:/openai/MediaGo-Builds/JW-Drama-Providers-20260905/win-unpacked';
const videoFile=path.join(output,'video-fixture.mp4');
const ffmpeg=spawnSync(path.join(oldBuild,'resources','tools','ffmpeg','ffmpeg.exe'),['-hide_banner','-loglevel','error','-f','lavfi','-i','color=c=blue:s=32x32:d=0.1','-an','-c:v','libx264','-pix_fmt','yuv420p',videoFile],{windowsHide:true});
assert.equal(ffmpeg.status,0,'failed to create local video fixture');const videoBytes=readFileSync(videoFile);
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j/p8AAAAASUVORK5CYII=', 'base64');
const wav = Buffer.alloc(8044); wav.write('RIFF',0); wav.writeUInt32LE(8036,4); wav.write('WAVEfmt ',8); wav.writeUInt32LE(16,16); wav.writeUInt16LE(1,20); wav.writeUInt16LE(1,22); wav.writeUInt32LE(8000,24); wav.writeUInt32LE(16000,28); wav.writeUInt16LE(2,32); wav.writeUInt16LE(16,34); wav.write('data',36); wav.writeUInt32LE(8000,40);
const calls = [];
const mock = http.createServer(async (req,res) => {
 let body=''; for await(const chunk of req) body += chunk;
 if(req.url.startsWith('/v1/')) assert.equal(req.headers.authorization,'Bearer jw-local-fixture-key');
 calls.push({method:req.method,path:req.url});
 res.setHeader('Content-Type','application/json');
 if(req.url==='/v1/models') return res.end(JSON.stringify({data:[{id:'gpt-image-fixture'},{id:'tts-fixture'},{id:'sora-fixture'},{id:'text-fixture'}]}));
 if(req.url==='/v1/images/generations') { assert.equal(JSON.parse(body).model,'gpt-image-fixture'); return res.end(JSON.stringify({data:[{b64_json:png.toString('base64')}]})); }
 if(req.url==='/v1/audio/speech') { assert.equal(JSON.parse(body).model,'tts-fixture'); res.setHeader('Content-Type','audio/wav'); return res.end(wav); }
 if(req.url==='/v1/videos') return res.end(JSON.stringify({id:'video-fixture',status:'queued',model:'sora-fixture'}));
 if(req.url==='/v1/videos/video-fixture') return res.end(JSON.stringify({id:'video-fixture',status:'completed',model:'sora-fixture',url:`http://127.0.0.1:${mock.address().port}/video.mp4`}));
 if(req.url==='/video.mp4'){res.setHeader('Content-Type','video/mp4');return res.end(videoBytes);}
 res.statusCode=404;res.end('{}');
});
await new Promise(resolve=>mock.listen(0,'127.0.0.1',resolve));
const portProbe=net.createServer();await new Promise(resolve=>portProbe.listen(0,'127.0.0.1',resolve));const port=portProbe.address().port;await new Promise(resolve=>portProbe.close(resolve));
const server=spawn(process.env.JW_MEDIA_TEST_SERVER || 'D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905/desktop/win-unpacked/resources/bin/mediago-server.exe',[],{windowsHide:true,cwd:output,env:{...process.env,
 MEDIAGO_WORKSPACE_DIR:path.join(output,'workspace'), MEDIAGO_SERVER_PORT:String(port), MEDIAGO_AGENT_BIN_DIR:process.env.JW_MEDIA_TEST_AGENTS || path.join(oldBuild,'resources','agents'), MEDIAGO_AGENT_ID:'codex', MEDIAGO_GENERATION_CLIS:'none', MEDIAGO_MODEL_PLATFORM:'none',
 CODEX_HOME:path.join(oldBuild,'data','workspace','.codex'),
}});
let logs='';server.stdout.on('data',d=>logs+=d);server.stderr.on('data',d=>logs+=d);
const base=`http://127.0.0.1:${port}`;
async function api(route,method='GET',body){const response=await fetch(base+'/api/v1'+route,{method,headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(45000)});const result=await response.json();assert.ok(response.ok&&result.success,`${route}: ${response.status} ${result.message}`);return result.data;}
async function generate(route,real=false){
 const session=await api('/generation/sessions','POST',{kind:route.kind,title:`JW ${real?'subscription':'fixture'} media verification`});
 let task=await api(`/generation/sessions/${session.sessionId}/messages`,'POST',{kind:route.kind,routeId:route.id,prompt:real?'Generate exactly one simple image: a blue ceramic cup on a plain white background. No text. Use image_generation.':'JW local protocol fixture'});
 const id=task.id;assert.ok(id,'missing task id');
 const deadline=Date.now()+(real?900000:20000);
 while(Date.now()<deadline && !['completed','failed','canceled'].includes(task.status)){await delay(real?2000:250);task=await api(`/generation/tasks/${encodeURIComponent(id)}`);if(route.kind==='video'&&task.providerTaskId)break;}
 if(route.kind==='video'){assert.equal(task.routeId,route.id);assert.equal(task.providerTaskId,route.id+':video-fixture');task=await api(`/generation/tasks/${encodeURIComponent(id)}/result`);}
 assert.equal(task.status,'completed',task.error||task.message||'generation did not complete');assert.ok(task.assets.length,'missing saved asset');
 const url=task.assets[0].url;assert.ok(url.startsWith('/api/v1/media-assets/'),'image was not saved locally');const asset=await fetch(base+url);assert.equal(asset.status,200);const bytes=Buffer.from(await asset.arrayBuffer());assert.ok(bytes.length>0);
 if(!real)assert.deepEqual(bytes,route.kind==='image'?png:route.kind==='audio'?wav:videoBytes);
 if(real)writeFileSync(path.join(output,'codex-subscription-test.png'),bytes);
 return {kind:route.kind,taskId:id,status:task.status,assetURL:url,bytes:bytes.length};
}
const report={output,realCodexGenerationRequested:process.argv.includes('--codex-generate'),results:[]};
try{
 let ready=false;for(let i=0;i<100;i++){try{await api('/health');ready=true;break;}catch{if(server.exitCode!==null)throw new Error('server exited before health check');await delay(250);}}assert.ok(ready,'server failed to start');
 await api('/settings/aihubmix','PUT',{baseUrl:`http://127.0.0.1:${mock.address().port}/v1`});
 await api('/settings/api-keys/aihubmix','PUT',{apiKey:'jw-local-fixture-key'});
 const catalog=await api('/generation/models');
 for(const kind of ['image','audio','video']){const route=catalog.routes.find(r=>r.provider==='aihubmix'&&r.kind===kind&&r.configured);assert.ok(route,`${kind} workbench missing model`);assert.ok(catalog.versions.some(v=>v.id===route.versionId));assert.ok(catalog.families.some(f=>f.id===route.familyId));report.results.push(await generate(route));console.log(`${kind} workbench: model selectable, request dispatched, result checked`);}
 const codex=catalog.routes.find(r=>r.provider==='codex-image'&&r.configured);report.codexImageSelectable=!!codex;assert.ok(codex,'logged-in native Codex image route missing');console.log('Codex subscription image: available in workbench catalog');
 const secondAudio=await generate(catalog.routes.find(r=>r.provider==='aihubmix'&&r.kind==='audio'&&r.configured));assert.notEqual(secondAudio.taskId,report.results.find(r=>r.kind==='audio').taskId,'repeated audio overwrote its previous task');report.repeatedAudioHasUniqueTaskID=true;
 if(report.realCodexGenerationRequested)report.results.push(await generate(codex,true));
 await api('/settings/api-keys/aihubmix','DELETE');const cleared=await api('/generation/models');assert.equal(cleared.routes.filter(r=>r.provider==='aihubmix'&&r.configured).length,0);assert.ok(cleared.routes.some(r=>r.provider==='codex-image'&&r.configured));
 report.calls=calls;report.passed=true;console.log(JSON.stringify(report,null,2));
}catch(error){report.passed=false;report.error=String(error);console.error(report.error);process.exitCode=1;}
finally{report.calls=calls;writeFileSync(path.join(output,'verification.json'),JSON.stringify(report,null,2));writeFileSync(path.join(output,'server-test.log'),logs);server.kill();mock.close();}
