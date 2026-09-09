// Time & Sales. Newest print at the top. Crosshair highlight is the
// prints that belong to the hovered bar — the same millisecond as
// the candle, CVD, and footprint cell.
(function () {
    const el = document.getElementById("tape");
    const rows = [];
    const maxRows = 400;

    function sideName(side) {
        if (side === 1) return "bid";
        if (side === 2) return "ask";
        return "none";
    }

    function fmtTime(ns) {
        if (!ns) return "";
        const d = new Date(ns / 1e6);
        return d.toLocaleTimeString("en-GB", {timeZone: "America/Chicago", hour12: false});
    }

    function render(highlightBar) {
        if (!el) return;
        const inst = (window.MDL && MDL.instrument && MDL.instrument()) || {};
        const tick = inst.TickSizeNano || 250000000;

        function px(t) {
            return (Number(t) * Number(tick) / 1e9).toFixed(2);
        }

        const bits = [];
        for (let i = 0; i < rows.length; i++) {
            const r = rows[i];
            const cls = (r.side === 1 ? "buy" : r.side === 2 ? "sell" : "") +
                (r.bar === highlightBar ? " on" : "");
            bits.push('<div class="' + cls + '">' +
                fmtTime(r.ts) + "  " + px(r.px) + "  x" + r.qty + "  " + sideName(r.side) +
                "</div>");
        }
        el.innerHTML = bits.join("");
    }

    function push(print, barIndex) {
        if (!print) return;
        rows.unshift({
            ts: print.TsEvent,
            px: print.Px,
            qty: print.Qty,
            side: print.Side,
            bar: barIndex,
        });
        if (rows.length > maxRows) rows.pop();
        render(-1);
    }

    function reset() {
        rows.length = 0;
        render(-1);
    }

    function highlight(barIndex) {
        render(barIndex);
    }

    window.MDL = window.MDL || {};
    window.MDL.tape = {push: push, reset: reset, highlight: highlight};
})();
