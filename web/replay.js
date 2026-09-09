// Live replay over /api/replay. The engine stays on one goroutine;
// this file only applies FrameBar upserts. requestAnimationFrame in
// chart.js coalesces draws so a speed-0 burst is not a slide show.
(function () {
    const btnPlay = document.getElementById("btn-play");
    const btnPause = document.getElementById("btn-pause");
    const btnRTH = document.getElementById("btn-rth");
    const btnETH = document.getElementById("btn-eth");
    const speedEl = document.getElementById("speed");
    const btnFull = document.getElementById("btn-full");

    let session = new URLSearchParams(location.search).get("session") || "RTH";
    let ws = null;

    function proto() {
        return location.protocol === "https:" ? "wss:" : "ws:";
    }

    function send(obj) {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify(obj));
        }
    }

    function connect() {
        if (ws) {
            ws.onclose = null;
            ws.close();
        }
        MDL.showErr("");
        btnRTH.setAttribute("aria-pressed", session === "RTH" ? "true" : "false");
        btnETH.setAttribute("aria-pressed", session === "ETH" ? "true" : "false");
        ws = new WebSocket(proto() + "//" + location.host + "/api/replay?session=" + encodeURIComponent(session));
        ws.onmessage = function (ev) {
            const f = JSON.parse(ev.data);
            if (f.type === "hello") {
                MDL.reset(f.Instrument, f.Header);
                send({type: "speed", speed: Number(speedEl.value)});
                return;
            }
            if (f.type === "bar" && f.bar) {
                MDL.applyBar(f.index, f.bar, f.vwap, f.cvd, {
                    footprint: f.Footprint,
                    profile: f.Profile,
                    tpo: f.TPO,
                    trade: f.trade,
                });
                return;
            }
            if (f.type === "error") {
                MDL.showErr(f.error || "replay error");
                return;
            }
            if (f.type === "done") {
                if (MDL.applyFlow) MDL.applyFlow({profile: f.Profile, tpo: f.TPO});
                MDL.setFollow(false);
            }
        };
        ws.onerror = function () {
            MDL.showErr("websocket error");
        };
        ws.onclose = function () {
            MDL.showErr("replay closed");
        };
    }

    btnPlay.addEventListener("click", function () {
        MDL.setFollow(true);
        send({type: "play"});
    });
    btnPause.addEventListener("click", function () {
        send({type: "pause"});
    });
    speedEl.addEventListener("change", function () {
        send({type: "speed", speed: Number(speedEl.value)});
    });
    btnRTH.addEventListener("click", function () {
        session = "RTH";
        connect();
    });
    btnETH.addEventListener("click", function () {
        session = "ETH";
        connect();
    });
    if (btnFull) {
        btnFull.addEventListener("click", async function () {
            MDL.showErr("");
            const r = await fetch("/api/view?session=" + encodeURIComponent(session));
            if (!r.ok) {
                MDL.showErr(await r.text());
                return;
            }
            MDL.loadView(await r.json());
        });
    }

    connect();
})();
