function (timestampSec) {
    return new Date(1000*timestampSec).toLocaleString('en-US');
}
