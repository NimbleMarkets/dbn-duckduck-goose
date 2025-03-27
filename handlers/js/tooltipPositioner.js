// https://echarts.apache.org/examples/en/editor.html?c=candlestick-brush
// Keeps the tooltip block in either corner of the chart
function (pos, params, el, elRect, size) {
    const obj = {
        top: 10
    };
    obj[['left', 'right'][+(pos[0] < size.viewSize[0] / 2)]] = 30;
    return obj;
}