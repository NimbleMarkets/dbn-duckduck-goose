function (value, valueStr) {    
    const zoomLabelFormatter = new Intl.DateTimeFormat('en-US', {
        year: 'numeric',
        month: 'numeric',
        day: 'numeric',
        hour: 'numeric',
        minute: 'numeric',
        second: 'numeric'
    });
    // valueStr is seconds since epoch, need milliseconds
    var dt = new Date(1000*valueStr);
    var dtStr = zoomLabelFormatter.format(dt);
    return dtStr;
}