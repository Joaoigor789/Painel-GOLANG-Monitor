package main

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime"

	"github.com/gorilla/sessions"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

var store = sessions.NewCookieStore([]byte("segredo-super-forte"))

type Stats struct {
	CPU       float64 `json:"cpu"`
	RAM       float64 `json:"ram"`
	Disk      float64 `json:"disk"`
	NetSent   uint64  `json:"net_sent"`
	NetRecv   uint64  `json:"net_recv"`
	Temp      float64 `json:"temp"`
	Hostname  string  `json:"hostname"`
	AlertHigh bool    `json:"alert_high"`
}

// ================= DISCO =================
func getDiskPath() string {
	if runtime.GOOS == "windows" {
		return "C:"
	}
	return "/"
}

// ================= TEMPERATURA =================
func getTemperature() float64 {
	temps, err := host.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return -1
	}
	return temps[0].Temperature
}

// ================= STATS =================
func getStats() Stats {
	cpuPercent, _ := cpu.Percent(0, false)
	memInfo, _ := mem.VirtualMemory()
	diskInfo, _ := disk.Usage(getDiskPath())
	netInfo, _ := net.IOCounters(false)
	hostInfo, _ := host.Info()

	var cpuVal float64
	if len(cpuPercent) > 0 {
		cpuVal = cpuPercent[0]
	}

	return Stats{
		CPU:       cpuVal,
		RAM:       memInfo.UsedPercent,
		Disk:      diskInfo.UsedPercent,
		NetSent:   netInfo[0].BytesSent,
		NetRecv:   netInfo[0].BytesRecv,
		Temp:      getTemperature(),
		Hostname:  hostInfo.Hostname,
		AlertHigh: cpuVal > 80,
	}
}

// ================= AUTH =================
func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session")

		if session.Values["auth"] != true {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// ================= HANDLERS =================
func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := getStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	session.Values["auth"] = false
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		user := r.FormValue("user")
		pass := r.FormValue("pass")

		if user == "admin" && pass == "1234" {
			session, _ := store.Get(r, "session")
			session.Values["auth"] = true
			session.Options.HttpOnly = true
			session.Options.Path = "/"
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}

	w.Write([]byte(loginPage))
}

// ================= MAIN =================
func main() {
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/stats", auth(statsHandler))
	http.HandleFunc("/", auth(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(htmlPage))
	}))

	log.Println(" Monitor PRO em http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// ================= LOGIN PAGE =================
var loginPage = `
<!DOCTYPE html>
<html lang="pt-br">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Login</title>
<style>
body{
background:#0f172a;
color:white;
font-family:Arial;
display:flex;
justify-content:center;
align-items:center;
height:100vh;
margin:0;
}
.box{
background:#111827;
padding:30px;
border-radius:12px;
text-align:center;
width:260px;
}
input{
width:100%;
padding:8px;
margin:6px 0;
border-radius:6px;
border:none;
}
button{
width:100%;
padding:10px;
background:#ef4444;
border:none;
color:white;
border-radius:6px;
cursor:pointer;
}
</style>
</head>
<body>
<div class="box">
<h2> Login</h2>
<form method="POST" action="/login">
<input name="user" placeholder="Usuário" required>
<input name="pass" type="password" placeholder="Senha" required>
<button type="submit">Entrar</button>
</form>
</div>
</body>
</html>
`

// ================= DASHBOARD =================
var htmlPage = `
<!DOCTYPE html>
<html lang="pt-br">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Monitor PRO</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>

<style>
body{background:#0f172a;color:white;font-family:Arial;margin:0;padding:10px}
.topbar{display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin-top:15px}
.card{background:#111827;padding:15px;border-radius:10px;text-align:center}
.alert{background:#ef4444;padding:10px;margin-top:10px;border-radius:8px;display:none;text-align:center;font-weight:bold}
canvas{margin-top:20px;background:#020617;border-radius:12px;width:100%!important;height:320px!important}
button{background:#ef4444;border:none;padding:8px 14px;color:white;border-radius:6px;cursor:pointer}
</style>
</head>
<body>

<div class="topbar">
<h2>☁️ Monitor PRO</h2>
<a href="/logout"><button>Logout</button></a>
</div>

<div id="alert" class="alert"> CPU acima de 80%</div>

<div class="grid">
<div class="card">CPU<br><b id="cpu">0%</b></div>
<div class="card">RAM<br><b id="ram">0%</b></div>
<div class="card">DISCO<br><b id="disk">0%</b></div>
<div class="card">TEMP<br><b id="temp">N/A</b></div>
<div class="card">NET ↑<br><b id="sent">0</b></div>
<div class="card">NET ↓<br><b id="recv">0</b></div>
</div>

<canvas id="chart"></canvas>

<script>
const ctx=document.getElementById('chart').getContext('2d');

const data={
labels:[],
datasets:[{
label:'CPU %',
data:[],
borderColor:'#ef4444',
tension:0.35,
fill:false
}]
};

const chart=new Chart(ctx,{
type:'line',
data:data,
options:{
responsive:true,
maintainAspectRatio:false,
animation:false,
scales:{y:{min:0,max:100}}
}
});

function formatBytes(bytes){
return (bytes/1024).toFixed(1)+' KB';
}

async function update(){
try{
const r=await fetch('/stats');
const s=await r.json();

cpu.innerText=s.cpu.toFixed(1)+'%';
ram.innerText=s.ram.toFixed(1)+'%';
disk.innerText=s.disk.toFixed(1)+'%';
sent.innerText=formatBytes(s.net_sent);
recv.innerText=formatBytes(s.net_recv);
temp.innerText=s.temp<0?'N/A':s.temp.toFixed(1)+'°C';
alert.style.display=s.alert_high?'block':'none';

data.labels.push(new Date().toLocaleTimeString());
data.datasets[0].data.push(s.cpu);

if(data.labels.length>60){
data.labels.shift();
data.datasets[0].data.shift();
}

chart.update();
}catch(e){console.log(e)}
}

setInterval(update,2000);
update();
</script>

</body>
</html>
`
