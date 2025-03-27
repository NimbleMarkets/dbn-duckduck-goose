function (params, ticket) {
    function pad(n, width) {
        n = n + '';
        return n.length >= width ? n :
            new Array(width - n.length + 1).join('0') + n;
    }

    function makeDateStr(time_ms) {
        // 2024-04-20 00:00:00.000
        let dt = new Date(time_ms);
        let dtStr = `${dt.getFullYear()}-${pad(dt.getMonth() + 1, 2)}-${pad(dt.getDate(), 2)}`;
        let timeStr = `${pad(dt.getHours(), 2)}:${pad(dt.getMinutes(), 2)}:${pad(dt.getSeconds(), 2)}`;
        return dtStr + ' ' + timeStr;
    }

    function addCandle(param) {
        let data = param.data;
        let str = `${param.marker}${param.seriesName}<br>${makeDateStr(1000*data.ts)}<br>`;
        str += `<tr><td>open:</td><td>${data.open.toFixed(2)}</td></tr>`;
        str += `<tr><td>high:</td><td>${data.high.toFixed(2)}</td></tr>`;
        str += `<tr><td>low:</td><td>${data.low.toFixed(2)}</td></tr>`;
        str += `<tr><td>close:</td><td>${data.close.toFixed(2)}</td></tr>`;
        str += `<tr><td>volume:</td><td>${data.volume}</td></tr>`;
        return str
    }

    function addLine(param) {
        let data = param.data;
        if (param.axisDim === 'y') {
            return `<tr><td>${param.marker}${param.seriesName}</td><td>${param.axisValue.toFixed(2)}</td></tr>`
        }
        return ''
    }

    // handle any candlestick series, if present
    let tooltip = `<table>`;
    for (let i = 0; i < params.length; i++) {
        let param = params[i];
        if (param.componentSubType === 'candlestick') {
            tooltip += addCandle(param);
            break;
        }
    }

    // now handle the line series, if present
    for (let i = 0; i < params.length; i++) {
        let param = params[i];
        if (param.componentSubType === 'line') {
            tooltip += addLine(param);
        } 
    }
    tooltip += `</table>`;
    return tooltip;
}