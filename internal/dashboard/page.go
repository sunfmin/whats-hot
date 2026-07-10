package dashboard

import (
	"context"
	"io"

	h "github.com/theplant/htmlgo"
)

// page renders the dashboard shell with htmlgo: a static layout plus inline CSS and JS.
// The live numbers are filled client-side from the /events SSE stream — htmlgo renders
// once, SSE does the rest (see docs/adr/0002-two-surfaces-one-core.md).
func page() h.HTMLComponent {
	return h.HTML(
		h.Head(
			h.Meta().Attr("charset", "utf-8"),
			h.Meta().Attr("name", "viewport", "content", "width=device-width, initial-scale=1"),
			h.Title("netmon — live network activity"),
			h.Style(css),
		),
		h.Body(
			h.Div(
				h.Div(h.Text("netmon")).Class("brand"),
				h.Div(h.Text("live network activity — reverse-DNS hostnames, no full URLs")).Class("sub"),
			).Class("header"),
			h.Div(
				gauge("down", "↓ download"),
				gauge("up", "↑ upload"),
			).Class("gauges"),
			h.Tag("canvas").Attr("id", "chart", "height", "80").Class("chart"),
			h.Div(
				h.Label("").Class("opt").Children(
					h.Input("").Attr("type", "checkbox").Id("opt-idle"),
					h.Span("隐藏空闲的进程 / 连接 (0 B/s)"),
				),
				h.Label("").Class("opt").Children(
					h.Input("").Attr("type", "checkbox").Id("opt-proc-only"),
					h.Span("仅显示进程（隐藏连接）"),
				),
			).Class("controls"),
			h.Table(
				h.Thead(h.Tr(
					h.Th("process / remote endpoint").Class("sortable").Id("th-name").Attr("data-sort", "name"),
					h.Th("↓ down").Class("n sortable").Id("th-down").Attr("data-sort", "down"),
					h.Th("↑ up").Class("n sortable").Id("th-up").Attr("data-sort", "up"),
					h.Th("flows").Class("n"),
				)),
				h.Tbody().Id("proc-body"),
			).Class("grid"),
			h.Div(h.Text("connecting…")).Id("status").Class("status"),
			h.Script(js),
		),
	)
}

func gauge(id, label string) h.HTMLComponent {
	return h.Div(
		h.Div(h.Text(label)).Class("label"),
		h.Div(h.Text("—")).Id("total-"+id).Class("value"),
	).Class("gauge " + id)
}

// Render writes the dashboard HTML to w.
func Render(w io.Writer) error {
	return h.Fprint(w, page(), context.Background())
}

const css = `
:root{color-scheme:dark light}
*{box-sizing:border-box}
body{margin:0;font:14px/1.4 -apple-system,BlinkMacSystemFont,"SF Pro Text",system-ui,sans-serif;
  background:#0b0e14;color:#e6e6e6;padding:20px}
.header{margin-bottom:16px}
.brand{font-size:22px;font-weight:700;letter-spacing:-.02em}
.sub{color:#8a94a6;font-size:12px}
.gauges{display:flex;gap:14px;margin-bottom:14px}
.gauge{flex:1;background:#141924;border:1px solid #222a3a;border-radius:12px;padding:14px 16px}
.gauge .label{color:#8a94a6;font-size:12px}
.gauge .value{font-size:28px;font-weight:700;font-variant-numeric:tabular-nums;margin-top:2px}
.gauge.down .value{color:#39d353}
.gauge.up .value{color:#4aa3ff}
.chart{width:100%;background:#141924;border:1px solid #222a3a;border-radius:12px;margin-bottom:14px;display:block}
.controls{display:flex;gap:18px;align-items:center;margin-bottom:10px}
.controls .opt{display:flex;gap:6px;align-items:center;cursor:pointer;user-select:none;
  color:#8a94a6;font-size:12px}
.controls input{accent-color:#4aa3ff;cursor:pointer;margin:0}
table.grid{width:100%;border-collapse:collapse}
table.grid th{text-align:left;color:#8a94a6;font-weight:600;font-size:11px;text-transform:uppercase;
  letter-spacing:.04em;padding:6px 10px;border-bottom:1px solid #222a3a}
th.sortable{cursor:pointer;user-select:none}
th.sortable:hover{color:#e6e6e6}
th.sortable[data-dir=asc]::after{content:" ▲";font-size:9px;color:#4aa3ff}
th.sortable[data-dir=desc]::after{content:" ▼";font-size:9px;color:#4aa3ff}
table.grid td{padding:5px 10px;border-bottom:1px solid #161c28;font-variant-numeric:tabular-nums}
th.n,td.n{text-align:right;white-space:nowrap}
tr.proc td{font-weight:600}
tr.proc .pid{color:#5b6577;font-weight:400;font-size:11px}
tr.flow td.ep{color:#b8c0d0;padding-left:22px;font-weight:400}
tr.flow .ip{display:block;color:#5b6577;font-size:11px}
tr.idle{opacity:.5}
tr.idle td.ep{color:#8a94a6}
.status{color:#5b6577;font-size:11px;margin-top:12px}
`

const js = `
const MAX=120, hist=[];
const $=id=>document.getElementById(id);
function fmt(bps){const u=['B/s','KB/s','MB/s','GB/s'];let v=bps||0,i=0;
  while(v>=1024&&i<u.length-1){v/=1024;i++;}return (i===0?v.toFixed(0):v.toFixed(v<10?1:0))+' '+u[i];}
function esc(s){return String(s).replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));}
function drawChart(){const c=$('chart');if(!c.getContext)return;const dpr=devicePixelRatio||1;
  const w=c.clientWidth,hgt=c.height;c.width=w*dpr;const x=c.getContext('2d');x.scale(dpr,1);
  x.clearRect(0,0,w,hgt);const max=Math.max(1024,...hist.flat());
  const line=(sel,col)=>{x.beginPath();hist.forEach((p,i)=>{const px=w*i/MAX,py=hgt-(sel(p)/max)*(hgt-6)-3;
    i?x.lineTo(px,py):x.moveTo(px,py);});x.strokeStyle=col;x.lineWidth=1.5;x.stroke();};
  line(p=>p[0],'#39d353');line(p=>p[1],'#4aa3ff');}
let last=null;
let sortKey=localStorage.getItem('sortKey')||'down';
let sortDir=localStorage.getItem('sortDir')||'desc';
function keyOf(x,k){return k==='name'?(x.name||x.remote&&(x.remote.host||x.remote.ip)||''):(k==='up'?(x.up_bps||0):(x.down_bps||0));}
function cmp(a,b){const dir=sortDir==='asc'?1:-1;const ka=keyOf(a,sortKey),kb=keyOf(b,sortKey);
  return dir*(sortKey==='name'?String(ka).localeCompare(String(kb)):(ka-kb));}
function syncSortUI(){['th-name','th-down','th-up'].forEach(id=>{const el=$(id);
  el.setAttribute('data-dir',el.getAttribute('data-sort')===sortKey?sortDir:'');});}
function paint(){
  const s=last;if(!s)return;
  const hideIdle=$('opt-idle').checked, procOnly=$('opt-proc-only').checked;
  const tb=$('proc-body');tb.innerHTML='';let shown=0;
  (s.processes||[]).slice().sort(cmp).forEach(p=>{
    let flows=(p.flows||[]).filter(f=>f.state==='Established');
    if(hideIdle)flows=flows.filter(f=>f.down_bps||f.up_bps);
    flows.sort(cmp);
    const moving=p.down_bps||p.up_bps;
    // hide idle: keep only processes moving bytes. otherwise: moving OR holding a connection.
    if(hideIdle?!moving:(!moving&&flows.length===0))return;
    shown++;
    const tr=document.createElement('tr');tr.className='proc'+(moving?'':' idle');
    tr.innerHTML='<td>'+esc(p.name)+' <span class=pid>'+p.pid+'</span></td>'+
      '<td class=n>'+fmt(p.down_bps)+'</td><td class=n>'+fmt(p.up_bps)+'</td><td class=n>'+flows.length+'</td>';
    tb.appendChild(tr);
    if(procOnly)return;
    flows.forEach(f=>{
      const fr=document.createElement('tr');
      const idle=!(f.down_bps||f.up_bps);fr.className='flow'+(idle?' idle':'');
      const host=f.remote.host||f.remote.ip;
      fr.innerHTML='<td class=ep>↳ '+esc(host)+
        '<span class=ip>'+esc(f.remote.ip)+':'+f.remote.port+'  '+esc(f.proto)+'  '+esc(f.state)+'</span></td>'+
        '<td class=n>'+fmt(f.down_bps)+'</td><td class=n>'+fmt(f.up_bps)+'</td><td></td>';
      tb.appendChild(fr);
    });
  });
  $('status').textContent=shown+' process(es) shown  ·  updated '+new Date().toLocaleTimeString();
}
function render(s){
  last=s;
  $('total-down').textContent=fmt(s.total_down_bps);
  $('total-up').textContent=fmt(s.total_up_bps);
  hist.push([s.total_down_bps||0,s.total_up_bps||0]);if(hist.length>MAX)hist.shift();drawChart();
  paint();
}
['opt-idle','opt-proc-only'].forEach(id=>{const el=$(id);
  el.checked=localStorage.getItem(id)==='1';
  el.addEventListener('change',()=>{localStorage.setItem(id,el.checked?'1':'0');paint();});});
['th-name','th-down','th-up'].forEach(id=>{$(id).addEventListener('click',()=>{
  const k=$(id).getAttribute('data-sort');
  if(sortKey===k)sortDir=sortDir==='asc'?'desc':'asc';
  else{sortKey=k;sortDir=k==='name'?'asc':'desc';}
  localStorage.setItem('sortKey',sortKey);localStorage.setItem('sortDir',sortDir);
  syncSortUI();paint();});});
syncSortUI();
const es=new EventSource('/events');
es.onmessage=e=>{try{render(JSON.parse(e.data));}catch(err){}};
es.onerror=()=>{$('status').textContent='disconnected — is netmon still running?';};
`
