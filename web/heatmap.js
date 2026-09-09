// Heatmap and DOM — same colors and widths as internal/chart/heatmap.go.
(function () {
    const bid = "#2563eb";
    const ask = "#dc2626";
    const last = "#d97706";
    const ink = "#222222";
    const domW = 80;

    function maxSize(hm, i0, i1) {
        let max = 0;
        if (!hm || !hm.Columns) return 0;
        const lastCol = Math.min(i1, hm.Columns.length);
        for (let i = i0; i < lastCol; i++) {
            const cells = hm.Columns[i] && hm.Columns[i].Cells;
            if (!cells) continue;
            for (let j = 0; j < cells.length; j++) {
                if (cells[j].Bid > max) max = cells[j].Bid;
                if (cells[j].Ask > max) max = cells[j].Ask;
            }
        }
        return max;
    }

    function cellH(yOf, step) {
        return Math.max(1, Math.abs(yOf(0) - yOf(step)));
    }

    function drawColumn(ctx, box, vis, yOf, hm, index, max) {
        if (!hm || !hm.Columns || index < 0 || index >= hm.Columns.length) return;
        const col = hm.Columns[index];
        if (!col || !col.Cells || !col.Cells.length || max <= 0) return;
        const step = hm.TicksPerRow > 0 ? hm.TicksPerRow : 1;
        const h = cellH(yOf, step);
        const slot = vis.n > 0 ? (box.right - box.left) / vis.n : 0;
        const half = Math.max(1, slot * 0.45);
        const x = box.left + (index - vis.i0 + 0.5) * slot;
        col.Cells.forEach(function (c) {
            const y = yOf(c.Price) - h / 2;
            if (c.Bid > 0) {
                ctx.fillStyle = bid;
                ctx.globalAlpha = 0.12 + 0.70 * c.Bid / max;
                ctx.fillRect(x - half, y, half, h);
            }
            if (c.Ask > 0) {
                ctx.fillStyle = ask;
                ctx.globalAlpha = 0.12 + 0.70 * c.Ask / max;
                ctx.fillRect(x, y, half, h);
            }
        });
        ctx.globalAlpha = 1;
    }

    function draw(ctx, box, vis, yOf, hm) {
        const max = maxSize(hm, vis.i0, vis.i1);
        if (max <= 0) return;
        for (let i = vis.i0; i < vis.i1; i++) {
            drawColumn(ctx, box, vis, yOf, hm, i, max);
        }
    }

    function drawDOM(ctx, box, yOf, d) {
        if (!d || !d.Levels || !d.Levels.length || !box.domW) return;
        const left = box.domLeft;
        const width = box.domW;
        const mid = left + width * 0.5;
        let max = 0;
        d.Levels.forEach(function (lv) {
            if (lv.Bid > max) max = lv.Bid;
            if (lv.Ask > max) max = lv.Ask;
        });
        const tickH = Math.max(8, Math.abs(yOf(0) - yOf(1)));
        const barW = width * 0.22;
        ctx.font = "8px ui-monospace, monospace";
        ctx.textAlign = "center";
        d.Levels.forEach(function (lv) {
            const y = yOf(lv.Price);
            if (max > 0 && lv.Bid > 0) {
                let w = barW * lv.Bid / max;
                if (w < 1) w = 1;
                ctx.globalAlpha = 0.45;
                ctx.fillStyle = bid;
                ctx.fillRect(mid - 4 - w, y - tickH / 2, w, tickH);
            }
            if (max > 0 && lv.Ask > 0) {
                let w = barW * lv.Ask / max;
                if (w < 1) w = 1;
                ctx.globalAlpha = 0.45;
                ctx.fillStyle = ask;
                ctx.fillRect(mid + 4, y - tickH / 2, w, tickH);
            }
            ctx.globalAlpha = 1;
            if (lv.Last > 0) {
                ctx.fillStyle = last;
                ctx.beginPath();
                ctx.arc(mid, y, 2.5, 0, Math.PI * 2);
                ctx.fill();
            }
        });
        ctx.textAlign = "left";
    }

    function atHeat(hm, barI, price) {
        if (!hm || !hm.Columns || barI < 0 || barI >= hm.Columns.length) {
            return {bid: 0, ask: 0};
        }
        const cells = hm.Columns[barI].Cells || [];
        const step = hm.TicksPerRow > 0 ? hm.TicksPerRow : 1;
        const g = price - (price % step);
        for (let i = 0; i < cells.length; i++) {
            if (cells[i].Price === g) return {bid: cells[i].Bid, ask: cells[i].Ask};
        }
        return {bid: 0, ask: 0};
    }

    function atDOM(d, price) {
        if (!d || !d.Levels) return {bid: 0, ask: 0, last: 0};
        for (let i = 0; i < d.Levels.length; i++) {
            if (d.Levels[i].Price === price) {
                return {bid: d.Levels[i].Bid, ask: d.Levels[i].Ask, last: d.Levels[i].Last};
            }
        }
        return {bid: 0, ask: 0, last: 0, lastPx: d.LastPx, lastQty: d.LastQty};
    }

    window.MDL = window.MDL || {};
    window.MDL.heatmap = {
        width: domW,
        draw: draw,
        drawColumn: function (ctx, box, vis, yOf, hm, index) {
            drawColumn(ctx, box, vis, yOf, hm, index, maxSize(hm, vis.i0, vis.i1));
        },
        drawDOM: drawDOM,
        atHeat: atHeat,
        atDOM: atDOM,
    };
})();
