// Canvas renderer for chart.View. X is the bar index, not wall time:
// a 15:30 bar and the 17:00 bar after the halt sit one slot apart,
// the same rule as internal/chart/scale.go. Footprint / TPO / profile
// stay on the SVG until step 18.
(function () {
  const canvas = document.getElementById("chart");
  const title = document.getElementById("title");
  const errBox = document.getElementById("err");
  const btnRTH = document.getElementById("btn-rth");
  const btnETH = document.getElementById("btn-eth");
  const ctx = canvas.getContext("2d");

  const padL = 16;
  const padR = 72;
  const padT = 16;
  const padB = 28;
  const up = "#1b7f3a";
  const down = "#b42318";
  const grid = "#e6e6e6";
  const ink = "#222222";
  const vwapColor = "#1d4ed8";
  const cvdColor = "#0f766e";

  const state = {
    view: null,
    session: new URLSearchParams(location.search).get("session") || "RTH",
    i0: 0,
    i1: 0,
    drag: null,
  };

  function showErr(msg) {
    errBox.textContent = msg;
    errBox.style.display = msg ? "block" : "none";
  }

  async function load(session) {
    showErr("");
    const r = await fetch("/api/view?session=" + encodeURIComponent(session));
    if (!r.ok) {
      showErr(await r.text());
      return;
    }
    const view = await r.json();
    state.view = view;
    state.session = session;
    state.i0 = 0;
    state.i1 = view.Bars ? view.Bars.length : 0;
    btnRTH.setAttribute("aria-pressed", session === "RTH" ? "true" : "false");
    btnETH.setAttribute("aria-pressed", session === "ETH" ? "true" : "false");
    const h = view.Header || {};
    const day = (h.TradingDate || "").slice(0, 10);
    title.textContent = [h.Symbol, day, h.Session].filter(Boolean).join("  ");
    draw();
  }

  function layout() {
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth;
    const h = canvas.clientHeight;
    canvas.width = Math.max(1, Math.floor(w * dpr));
    canvas.height = Math.max(1, Math.floor(h * dpr));
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const left = padL;
    const right = w - padR;
    const top = padT;
    const bottom = h - padB;
    const hasPanel = state.view && state.view.Panels && state.view.Panels.length > 0;
    const priceBottom = hasPanel ? bottom - (bottom - top) * 0.24 : bottom;
    return { w, h, left, right, top, bottom, priceBottom };
  }

  function visibleBars() {
    const bars = state.view && state.view.Bars ? state.view.Bars : [];
    const i0 = Math.max(0, Math.min(state.i0, bars.length));
    const i1 = Math.max(i0, Math.min(state.i1, bars.length));
    return { bars, i0, i1, n: i1 - i0 };
  }

  function xOf(i, box, vis) {
    if (vis.n <= 0) return box.left;
    const slot = (box.right - box.left) / vis.n;
    return box.left + (i - vis.i0 + 0.5) * slot;
  }

  function slotWidth(box, vis) {
    if (vis.n <= 0) return 0;
    return (box.right - box.left) / vis.n;
  }

  function priceBounds(vis) {
    let lo = Infinity;
    let hi = -Infinity;
    for (let i = vis.i0; i < vis.i1; i++) {
      const b = vis.bars[i];
      if (b.Low < lo) lo = b.Low;
      if (b.High > hi) hi = b.High;
    }
    if (!isFinite(lo)) return { lo: 0, hi: 1 };
    if (lo === hi) hi = lo + 1;
    const pad = Math.max(1, Math.round((hi - lo) * 0.08));
    return { lo: lo - pad, hi: hi + pad };
  }

  function yOf(px, box, pb) {
    if (pb.hi === pb.lo) return (box.top + box.priceBottom) / 2;
    const frac = (pb.hi - px) / (pb.hi - pb.lo);
    return box.top + frac * (box.priceBottom - box.top);
  }

  function formatPrice(inst, ticks) {
    const nano = inst && inst.TickSizeNano ? inst.TickSizeNano : 0;
    if (!nano) return String(ticks);
    return (Number(ticks) * Number(nano) / 1e9).toFixed(2);
  }

  function niceTicks(lo, hi, target) {
    const rng = hi - lo;
    if (rng <= 0) return [lo];
    const raw = rng / target;
    const exp = Math.pow(10, Math.floor(Math.log10(Math.max(raw, 1))));
    const frac = raw / exp;
    let nf = 10;
    if (frac <= 1) nf = 1;
    else if (frac <= 2) nf = 2;
    else if (frac <= 5) nf = 5;
    const step = Math.max(1, Math.round(nf * exp));
    let start = lo;
    const rem = start % step;
    if (rem !== 0) start = start - rem + (start >= 0 ? step : 0);
    const out = [];
    for (let t = start; t <= hi; t += step) out.push(t);
    return out.length ? out : [lo];
  }

  function draw() {
    if (!state.view) return;
    const box = layout();
    const vis = visibleBars();
    ctx.clearRect(0, 0, box.w, box.h);
    if (vis.n <= 0) {
      ctx.fillStyle = ink;
      ctx.fillText("no bars", box.left, box.top + 20);
      return;
    }
    const pb = priceBounds(vis);
    const inst = state.view.Instrument || {};

    ctx.strokeStyle = grid;
    ctx.lineWidth = 1;
    ctx.fillStyle = ink;
    ctx.font = "11px ui-monospace, monospace";
    niceTicks(pb.lo, pb.hi, 6).forEach(function (px) {
      const y = yOf(px, box, pb);
      ctx.beginPath();
      ctx.moveTo(box.left, y);
      ctx.lineTo(box.right, y);
      ctx.stroke();
      ctx.fillText(formatPrice(inst, px), box.right + 8, y + 4);
    });

    const hw = slotWidth(box, vis) * 0.3;
    for (let i = vis.i0; i < vis.i1; i++) {
      const b = vis.bars[i];
      const x = xOf(i, box, vis);
      const color = b.Close < b.Open ? down : up;
      ctx.strokeStyle = color;
      ctx.beginPath();
      ctx.moveTo(x, yOf(b.High, box, pb));
      ctx.lineTo(x, yOf(b.Low, box, pb));
      ctx.stroke();
      let top = yOf(b.Open, box, pb);
      let bot = yOf(b.Close, box, pb);
      if (top > bot) {
        const tmp = top;
        top = bot;
        bot = tmp;
      }
      const bh = Math.max(1, bot - top);
      ctx.fillStyle = color;
      ctx.fillRect(x - hw, top, hw * 2, bh);
    }

    const overlays = state.view.Overlays || [];
    overlays.forEach(function (s) {
      const vals = s.Values || [];
      ctx.strokeStyle = vwapColor;
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      let started = false;
      const last = Math.min(vals.length, vis.i1);
      for (let i = vis.i0; i < last; i++) {
        const x = xOf(i, box, vis);
        const y = yOf(vals[i], box, pb);
        if (!started) {
          ctx.moveTo(x, y);
          started = true;
        } else ctx.lineTo(x, y);
      }
      ctx.stroke();
    });

    if (state.view.Panels && state.view.Panels.length) {
      drawPanel(state.view.Panels[0], box, vis);
    }

    const step = vis.n > 12 ? Math.ceil(vis.n / 8) : 1;
    ctx.fillStyle = ink;
    ctx.font = "11px ui-monospace, monospace";
    for (let i = vis.i0; i < vis.i1; i += step) {
      const label = (vis.bars[i].Start || "").slice(11, 16);
      ctx.fillText(label, xOf(i, box, vis) - 14, box.bottom + 16);
    }
  }

  function drawPanel(panel, box, vis) {
    const series = panel.Series || [];
    let lo = 0;
    let hi = 0;
    let ok = false;
    series.forEach(function (s) {
      (s.Values || []).forEach(function (v) {
        if (!ok) {
          lo = hi = v;
          ok = true;
          return;
        }
        if (v < lo) lo = v;
        if (v > hi) hi = v;
      });
    });
    if (lo > 0) lo = 0;
    if (hi < 0) hi = 0;
    if (lo === hi) hi = lo + 1;
    const top = box.priceBottom + 10;
    const bottom = box.bottom;
    ctx.strokeStyle = grid;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(box.left, top);
    ctx.lineTo(box.right, top);
    ctx.stroke();
    ctx.fillStyle = ink;
    ctx.fillText(panel.Name || "", box.left, top + 12);
    function py(v) {
      const frac = (hi - v) / (hi - lo);
      return top + 16 + frac * (bottom - top - 16);
    }
    if (lo < 0 && hi > 0) {
      ctx.strokeStyle = "#9ca3af";
      ctx.beginPath();
      ctx.moveTo(box.left, py(0));
      ctx.lineTo(box.right, py(0));
      ctx.stroke();
    }
    series.forEach(function (s) {
      const vals = s.Values || [];
      ctx.strokeStyle = cvdColor;
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      let started = false;
      const last = Math.min(vals.length, vis.i1);
      for (let i = vis.i0; i < last; i++) {
        const x = xOf(i, box, vis);
        const y = py(vals[i]);
        if (!started) {
          ctx.moveTo(x, y);
          started = true;
        } else ctx.lineTo(x, y);
      }
      ctx.stroke();
    });
  }

  function clampWindow() {
    const n = state.view && state.view.Bars ? state.view.Bars.length : 0;
    if (state.i0 < 0) {
      state.i1 -= state.i0;
      state.i0 = 0;
    }
    if (state.i1 > n) {
      state.i0 -= state.i1 - n;
      state.i1 = n;
    }
    if (state.i0 < 0) state.i0 = 0;
    if (state.i1 - state.i0 < 5 && n >= 5) {
      state.i1 = Math.min(n, state.i0 + 5);
      state.i0 = Math.max(0, state.i1 - 5);
    }
  }

  canvas.addEventListener("wheel", function (e) {
    e.preventDefault();
    if (!state.view || !state.view.Bars) return;
    const box = layout();
    const vis = visibleBars();
    if (vis.n <= 0) return;
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const frac = (mx - box.left) / Math.max(1, box.right - box.left);
    const anchor = vis.i0 + frac * vis.n;
    const factor = e.deltaY > 0 ? 1.15 : 1 / 1.15;
    let n = Math.max(5, Math.round(vis.n * factor));
    n = Math.min(n, vis.bars.length);
    state.i0 = Math.round(anchor - frac * n);
    state.i1 = state.i0 + n;
    clampWindow();
    draw();
  }, { passive: false });

  canvas.addEventListener("pointerdown", function (e) {
    canvas.setPointerCapture(e.pointerId);
    state.drag = { x: e.clientX, i0: state.i0, i1: state.i1 };
  });
  canvas.addEventListener("pointerup", function () { state.drag = null; });
  canvas.addEventListener("pointercancel", function () { state.drag = null; });
  canvas.addEventListener("pointermove", function (e) {
    if (!state.drag || !state.view) return;
    const box = layout();
    const visN = state.drag.i1 - state.drag.i0;
    if (visN <= 0) return;
    const slot = (box.right - box.left) / visN;
    const dx = e.clientX - state.drag.x;
    const shift = Math.round(-dx / slot);
    state.i0 = state.drag.i0 + shift;
    state.i1 = state.drag.i1 + shift;
    clampWindow();
    draw();
  });
  canvas.addEventListener("dblclick", function () {
    if (!state.view || !state.view.Bars) return;
    state.i0 = 0;
    state.i1 = state.view.Bars.length;
    draw();
  });

  btnRTH.addEventListener("click", function () { load("RTH"); });
  btnETH.addEventListener("click", function () { load("ETH"); });
  window.addEventListener("resize", draw);

  load(state.session);
})();
