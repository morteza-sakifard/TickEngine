// Footprint cells — same geometry as internal/chart/footprint.go.
// Left = sell (SideAsk hit the bid), right = buy (SideBid lifted ask).
// Gold stroke is imbalance; thicker is stacked. Opacity is volume
// versus the max cell in that bar.
(function () {
  const up = "#1b7f3a";
  const down = "#b42318";
  const imb = "#eab308";

  function cellMark(imbs, px, dir) {
    imbs = imbs || [];
    for (let i = 0; i < imbs.length; i++) {
      if (imbs[i].Price === px && imbs[i].Dir === dir) {
        return { hit: true, stacked: !!imbs[i].Stacked };
      }
    }
    return { hit: false, stacked: false };
  }

  function draw(ctx, box, vis, yOf, fp) {
    if (!fp || !fp.Bars) return;
    const step = fp.TicksPerRow > 0 ? fp.TicksPerRow : 4;
    const h = Math.max(1, Math.abs(yOf(0) - yOf(step)));
    const slot = vis.n <= 0 ? 0 : (box.right - box.left) / vis.n;
    const half = Math.max(1, slot * 0.4);
    const n = Math.min(fp.Bars.length, vis.i1);
    for (let i = vis.i0; i < n; i++) {
      const bar = fp.Bars[i];
      if (!bar || !bar.Levels) continue;
      let maxV = 0;
      for (let k = 0; k < bar.Levels.length; k++) {
        if (bar.Levels[k].Buy > maxV) maxV = bar.Levels[k].Buy;
        if (bar.Levels[k].Sell > maxV) maxV = bar.Levels[k].Sell;
      }
      if (maxV <= 0) continue;
      const x = box.left + (i - vis.i0 + 0.5) * slot;
      for (let k = 0; k < bar.Levels.length; k++) {
        const lv = bar.Levels[k];
        const y = yOf(lv.Price) - h / 2;
        if (lv.Sell > 0) {
          const m = cellMark(bar.Imbs, lv.Price, 1);
          cell(ctx, x - half, y, half, h, down, lv.Sell, maxV, m);
        }
        if (lv.Buy > 0) {
          const m = cellMark(bar.Imbs, lv.Price, 2);
          cell(ctx, x, y, half, h, up, lv.Buy, maxV, m);
        }
      }
    }
  }

  function cell(ctx, x, y, w, h, color, vol, max, mark) {
    ctx.fillStyle = color;
    ctx.globalAlpha = 0.20 + 0.70 * vol / max;
    ctx.fillRect(x, y, w, h);
    ctx.globalAlpha = 1;
    if (mark.hit) {
      ctx.strokeStyle = imb;
      ctx.lineWidth = mark.stacked ? 2.5 : 1.5;
      ctx.strokeRect(x + 0.5, y + 0.5, Math.max(0, w - 1), Math.max(0, h - 1));
    }
  }

  function at(fp, barIndex, price) {
    if (!fp || !fp.Bars || barIndex < 0 || barIndex >= fp.Bars.length) {
      return { buy: 0, sell: 0 };
    }
    const bar = fp.Bars[barIndex];
    const step = fp.TicksPerRow > 0 ? fp.TicksPerRow : 4;
    const row = price - (price % step);
    let buy = 0;
    let sell = 0;
    (bar.Levels || []).forEach(function (lv) {
      if (lv.Price === row) {
        buy = lv.Buy;
        sell = lv.Sell;
      }
    });
    return { buy: buy, sell: sell };
  }

  window.MDL = window.MDL || {};
  window.MDL.footprint = { draw: draw, at: at };
})();
