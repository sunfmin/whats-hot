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
			h.Div().Id("alerts").Class("alerts"),
			h.Table(
				h.Thead(h.Tr(
					h.Th("process / remote endpoint"),
					h.Th("↓ down").Class("n"),
					h.Th("↑ up").Class("n"),
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
.alerts:empty{display:none}
.alerts{margin-bottom:12px}
.alert{background:#3a1d1d;border:1px solid #7a2e2e;color:#ffb4b4;padding:8px 12px;border-radius:8px;margin-bottom:6px;font-size:13px}
table.grid{width:100%;border-collapse:collapse}
table.grid th{text-align:left;color:#8a94a6;font-weight:600;font-size:11px;text-transform:uppercase;
  letter-spacing:.04em;padding:6px 10px;border-bottom:1px solid #222a3a}
table.grid td{padding:5px 10px;border-bottom:1px solid #161c28;font-variant-numeric:tabular-nums}
th.n,td.n{text-align:right;white-space:nowrap}
tr.proc td{font-weight:600}
tr.proc .pid{color:#5b6577;font-weight:400;font-size:11px}
tr.flow td.ep{color:#b8c0d0;padding-left:22px;font-weight:400}
tr.flow .ip{display:block;color:#5b6577;font-size:11px}
tr.stalled td.ep{color:#ffb4b4}
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
function render(s){
  $('total-down').textContent=fmt(s.total_down_bps);
  $('total-up').textContent=fmt(s.total_up_bps);
  hist.push([s.total_down_bps||0,s.total_up_bps||0]);if(hist.length>MAX)hist.shift();drawChart();
  const tb=$('proc-body');tb.innerHTML='';let active=0;
  (s.processes||[]).forEach(p=>{
    const flows=(p.flows||[]).filter(f=>f.state==='Established');
    if(!(p.down_bps||p.up_bps)&&flows.length===0)return;active++;
    const tr=document.createElement('tr');tr.className='proc';
    tr.innerHTML='<td>'+esc(p.name)+' <span class=pid>'+p.pid+'</span></td>'+
      '<td class=n>'+fmt(p.down_bps)+'</td><td class=n>'+fmt(p.up_bps)+'</td><td class=n>'+flows.length+'</td>';
    tb.appendChild(tr);
    flows.forEach(f=>{
      const fr=document.createElement('tr');
      const stalled=f.down_bps===0&&f.up_bps===0;fr.className='flow'+(stalled?' stalled':'');
      const host=f.remote.host||f.remote.ip;
      fr.innerHTML='<td class=ep>↳ '+esc(host)+
        '<span class=ip>'+esc(f.remote.ip)+':'+f.remote.port+'  '+esc(f.proto)+'  '+esc(f.state)+'</span></td>'+
        '<td class=n>'+fmt(f.down_bps)+'</td><td class=n>'+fmt(f.up_bps)+'</td><td></td>';
      tb.appendChild(fr);
    });
  });
  $('status').textContent=active+' active process(es)  ·  updated '+new Date().toLocaleTimeString();
}
const es=new EventSource('/events');
es.onmessage=e=>{try{render(JSON.parse(e.data));}catch(err){}};
es.onerror=()=>{$('status').textContent='disconnected — is netmon still running?';};
`
