// Package webui 提供拓扑画布页面：以 SVG 绘制节点与边，联动显示流程步骤与风险。
package webui

import (
	"fmt"
	"net/http"
	"strings"
)

// PageHTML 返回微流控拓扑画布页面。
// 页面通过 fetch 调用 /api 接口，把拓扑、流程步骤与风险路径联动渲染。
func PageHTML() string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>微流控芯片通道拓扑校验台</title>
<style>
body{font-family:-apple-system,"PingFang SC",sans-serif;margin:0;background:#f7f8fa;color:#222}
header{padding:14px 24px;background:#1f2937;color:#fff;display:flex;align-items:center;gap:16px}
header h1{font-size:17px;margin:0}
.wrap{padding:20px 24px;display:grid;grid-template-columns:1fr 320px;gap:16px;align-items:start}
.card{background:#fff;border:1px solid #e5e7eb;border-radius:8px;padding:16px;margin-bottom:16px}
.card h2{font-size:14px;margin:0 0 12px;color:#374151}
.meta{display:flex;gap:10px;flex-wrap:wrap}
.meta input,select{font-size:13px;padding:6px 8px;border:1px solid #d1d5db;border-radius:6px}
button{font-size:13px;padding:6px 12px;border:none;border-radius:6px;background:#2563eb;color:#fff;cursor:pointer}
button.sec{background:#6b7280}
#canvas{width:100%;height:560px;background:#fff;border:1px solid #e5e7eb;border-radius:8px}
.node{fill:#dbeafe;stroke:#2563eb}
.node.valve{fill:#fef3c7;stroke:#d97706}
.node.inlet{fill:#d1fae5;stroke:#059669}
.node.reaction_well{fill:#fce7f3;stroke:#db2777}
.node.waste_well{fill:#e5e7eb;stroke:#6b7280}
.edge{stroke:#9ca3af;stroke-width:2}
.edge.risk{stroke:#dc2626;stroke-width:3}
.edge.blocked{stroke:#dc2626;stroke-dasharray:5,4}
.label{font-size:10px;fill:#374151;pointer-events:none}
#log{font-family:ui-monospace,Menlo,monospace;font-size:12px;background:#111827;color:#34d399;border-radius:8px;padding:12px;height:340px;overflow:auto;white-space:pre-wrap}
table{width:100%;border-collapse:collapse;font-size:12px}
th,td{border:1px solid #e5e7eb;padding:6px 8px;text-align:left}
th{background:#f3f4f6}
.badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:11px}
.badge.ok{background:#d1fae5;color:#047857}
.badge.bad{background:#fee2e2;color:#b91c1c}
</style>
</head>
<body>
<header>
  <h1>微流控芯片通道拓扑校验台</h1>
  <span style="font-size:12px;color:#9ca3af">task169-microfluidbench</span>
</header>
<div class="wrap">
  <div>
    <div class="card">
      <h2>版本与拓扑</h2>
      <div class="meta">
        <select id="chipSel"></select>
        <select id="verSel"></select>
        <input id="nodeName" placeholder="节点名" value="N">
        <select id="nodeKind">
          <option value="inlet">入口</option><option value="outlet">出口</option>
          <option value="valve">阀门</option><option value="channel">通道</option>
          <option value="reaction_well">反应腔</option><option value="waste_well">废液腔</option>
        </select>
        <button onclick="addNode()">加节点</button>
        <button class="sec" onclick="refresh()">刷新</button>
        <button class="sec" onclick="validateAll()">校验全部</button>
      </div>
      <svg id="canvas" viewBox="0 0 900 520" preserveAspectRatio="xMidYMid meet"></svg>
    </div>
    <div class="card">
      <h2>流程步骤与风险路径联动</h2>
      <div id="stepsBox">（请先选择版本）</div>
    </div>
  </div>
  <div>
    <div class="card">
      <h2>风险与校验记录</h2>
      <div id="riskBox">（暂无）</div>
    </div>
    <div class="card">
      <h2>操作日志</h2>
      <div id="log">等待操作…</div>
    </div>
  </div>
</div>
<script>
var $=function(id){return document.getElementById(id);};
var log=function(m){$('log').textContent+=m+"\n";$('log').scrollTop=$('log').scrollHeight;};
var api=async function(u,o){o=o||{};o.headers=o.headers||{'Content-Type':'application/json'};var r=await fetch(u,o);var j=await r.json();if(!r.ok){throw new Error(j.error||r.status);}return j;};
var nodes=[],edges=[];
async function boot(){
  var chips=await api('/api/chips');
  var opts='';
  for(var i=0;i<chips.length;i++){opts+='<option value="'+chips[i].id+'">'+chips[i].name+'</option>';}
  $('chipSel').innerHTML=opts||'<option>—</option>';
  $('chipSel').onchange=loadVersions;
  if(chips.length){await loadVersions();}else{log('尚无芯片：POST /api/chips 创建');}
}
async function loadVersions(){
  var cid=$('chipSel').value; if(!cid){return;}
  var vs=await api('/api/chips/'+cid+'/versions');
  var opts='';
  for(var i=0;i<vs.length;i++){opts+='<option value="'+vs[i].id+'">v'+vs[i].version_no+' ['+vs[i].status+'] rev'+vs[i].rev+'</option>';}
  $('verSel').innerHTML=opts||'<option>—</option>';
  $('verSel').onchange=refresh;
  if(vs.length){await refresh();}
}
async function refresh(){
  var vid=$('verSel').value; if(!vid){return;}
  try{
    var g=await api('/api/versions/'+vid+'/graph');
    nodes=g.nodes;edges=g.edges;
    var risks=await api('/api/versions/'+vid+'/risks');
    draw(g,risks);
    renderSteps(vid);
    renderRisks(risks);
    log('已加载版本 '+vid+'：'+nodes.length+' 节点 / '+edges.length+' 边 / 风险 '+risks.length);
  }catch(e){log('加载失败: '+e.message);}
}
async function addNode(){
  var vid=$('verSel').value; if(!vid){return;}
  var name=$('nodeName').value||('N'+nodes.length);
  var x=80+Math.random()*700,y=60+Math.random()*380;
  await api('/api/versions/'+vid+'/nodes',{method:'POST',body:JSON.stringify({kind:$('nodeKind').value,name:name,x:x,y:y})});
  await refresh(); log('新增节点 '+name);
}
async function validateAll(){
  var vid=$('verSel').value; if(!vid){return;}
  var r=await api('/api/versions/'+vid+'/validate-all',{method:'POST',body:'{}'});
  log('校验全部: all_passed='+r.all_passed);
  await refresh();
}
function draw(g,risks){
  var svg=$('canvas'); svg.innerHTML='';
  var riskEdge={};
  for(var i=0;i<risks.length;i++){var ev=risks[i].evidence||'';var m=ev.match(/\d+/g);if(m){for(var j=0;j<m.length;j++){riskEdge[Number(m[j])]=true;}}}
  for(var k=0;k<g.edges.length;k++){
    var e=g.edges[k];
    var f=findNode(g.nodes,e.from_node_id),t=findNode(g.nodes,e.to_node_id);
    if(!f||!t){continue;}
    var l=document.createElementNS('http://www.w3.org/2000/svg','line');
    l.setAttribute('x1',f.x);l.setAttribute('y1',f.y);l.setAttribute('x2',t.x);l.setAttribute('y2',t.y);
    var w=(typeof e.width==='number'&&e.width>0)?Math.max(1,Math.min(12,e.width/2)):2;
    l.setAttribute('stroke-width',w);
    l.setAttribute('class','edge'+(riskEdge[e.id]?' risk':''));
    svg.appendChild(l);
  }
  for(var m2=0;m2<g.nodes.length;m2++){
    var n=g.nodes[m2];
    var grp=document.createElementNS('http://www.w3.org/2000/svg','g');
    var c=document.createElementNS('http://www.w3.org/2000/svg','circle');
    c.setAttribute('cx',n.x);c.setAttribute('cy',n.y);c.setAttribute('r',14);
    c.setAttribute('class','node '+n.kind);
    var t2=document.createElementNS('http://www.w3.org/2000/svg','text');
    t2.setAttribute('x',n.x);t2.setAttribute('y',n.y-18);t2.setAttribute('class','label');
    t2.textContent=n.name+'('+n.kind+')';
    grp.appendChild(c);grp.appendChild(t2);svg.appendChild(grp);
  }
}
function findNode(list,id){for(var i=0;i<list.length;i++){if(list[i].id===id){return list[i];}}return null;}
async function renderSteps(vid){
  var steps=await api('/api/versions/'+vid+'/steps');
  if(!steps.length){$('stepsBox').innerHTML='<p style="color:#6b7280">尚无流程步骤</p>';return;}
  var html='<table><tr><th>#</th><th>流体</th><th>入口</th><th>出口</th><th>状态</th><th>阀门</th></tr>';
  for(var i=0;i<steps.length;i++){
    var s=steps[i];
    var valves='';
    for(var j=0;j<(s.valves||[]).length;j++){valves+=s.valves[j].node_id+':'+s.valves[j].state+' ';}
    html+='<tr><td>'+s.order_no+'</td><td>'+s.fluid_type+'</td><td>'+s.inlet_id+'</td><td>'+s.outlet_id+'</td>'+
      '<td><span class="badge '+(s.status==='verified'?'ok':'bad')+'">'+s.status+'</span></td><td>'+valves+'</td></tr>';
  }
  $('stepsBox').innerHTML=html+'</table>';
}
async function renderRisks(risks){
  if(!risks.length){$('riskBox').innerHTML='<p style="color:#6b7280">暂无风险</p>';return;}
  var html='';
  for(var i=0;i<risks.length;i++){
    var r=risks[i];
    html+='<div style="padding:8px;border:1px solid #fee2e2;background:#fff7f7;border-radius:6px;margin-bottom:8px">'+
      '<b>['+r.severity+'] '+r.title+'</b> <span class="badge bad">'+r.kind+'</span><br>'+
      '<span style="font-size:12px;color:#6b7280">'+r.evidence+' · '+r.status+'</span></div>';
  }
  $('riskBox').innerHTML=html;
}
boot().catch(function(e){log('初始化失败: '+e.message);});
</script>
</body>
</html>`
}

// Handler 返回页面处理器。
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, strings.ReplaceAll(PageHTML(), "{{VERSION}}", "task169"))
	})
}
