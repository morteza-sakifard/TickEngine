// Volume profile and TPO — same colors and widths as
// internal/chart/profile.go and tpo.go. TPO lives here because both
// are right-side ladders on the same y-scale; the roadmap listed
// profile.js as the file for that side of the page.
(function () {
  const profileW = 88;
  const tpoW = 100;
  const va = "#93c5fd";
  const poc = "#d97706";
  const outer = "#d1d5db";
  const ib = "#fef3c7";
  const single = "#dc2626";
  const ink = "#222222";

  function drawProfile(ctx, box, yOf, p) {
    if (!p || !p.Levels || !p.Levels.length) return;
    let maxV = 0;
    p.Levels.forEach(function (lv) {
      if (lv.Volume > maxV) maxV = lv.Volume;
    });
    if (maxV <= 0) return;
    const left = box.profLeft;
    const width = box.profW;
    const tickH = Math.max(1, Math.abs(yOf(0) - yOf(1)));
    p.Levels.forEach(function (lv) {
      let barW = width * lv.Volume / maxV;
      if (barW < 1) barW = 1;
      let color = outer;
      if (lv.Price >= p.VAL && lv.Price <= p.VAH) color = va;
      if (lv.Price === p.POC) color = poc;
      ctx.fillStyle = color;
      ctx.fillRect(left + width - barW, yOf(lv.Price) - tickH / 2, barW, tickH);
    });
    const yPOC = yOf(p.POC);
    ctx.save();
    ctx.strokeStyle = poc;
    ctx.setLineDash([4, 3]);
    ctx.beginPath();
    ctx.moveTo(box.left, yPOC);
    ctx.lineTo(left + width, yPOC);
    ctx.stroke();
    ctx.restore();
  }

  function drawTPO(ctx, box, yOf, t) {
    if (!t || !t.Levels || !t.Levels.length) return;
    const left = box.tpoLeft;
    const width = box.tpoW;
    if (t.HasIB) {
      let y1 = yOf(t.IBHigh);
      let y2 = yOf(t.IBLow);
      if (y1 > y2) {
        const tmp = y1;
        y1 = y2;
        y2 = tmp;
      }
      ctx.fillStyle = ib;
      ctx.fillRect(left, y1, width, Math.max(1, y2 - y1));
    }
    ctx.save();
    ctx.beginPath();
    ctx.rect(left, box.top, width, box.priceBottom - box.top);
    ctx.clip();
    ctx.font = "8px ui-monospace, monospace";
    t.Levels.forEach(function (lv) {
      let color = ink;
      if (lv.Price === t.POC) color = poc;
      else if (lv.Single) color = single;
      ctx.fillStyle = color;
      ctx.fillText(lv.Letters || "", left + 2, yOf(lv.Price) + 3);
    });
    ctx.restore();
  }

  function atProfile(p, price) {
    if (!p || !p.Levels) return { volume: 0, poc: 0, val: 0, vah: 0 };
    let volume = 0;
    for (let i = 0; i < p.Levels.length; i++) {
      if (p.Levels[i].Price === price) {
        volume = p.Levels[i].Volume;
        break;
      }
    }
    return { volume: volume, poc: p.POC, val: p.VAL, vah: p.VAH };
  }

  function atTPO(t, price) {
    if (!t || !t.Levels) return { letters: "", poc: 0 };
    for (let i = 0; i < t.Levels.length; i++) {
      if (t.Levels[i].Price === price) {
        return { letters: t.Levels[i].Letters || "", poc: t.POC, single: !!t.Levels[i].Single };
      }
    }
    return { letters: "", poc: t ? t.POC : 0 };
  }

  window.MDL = window.MDL || {};
  window.MDL.profile = {
    width: profileW,
    tpoWidth: tpoW,
    drawProfile: drawProfile,
    drawTPO: drawTPO,
    atProfile: atProfile,
    atTPO: atTPO,
  };
})();
