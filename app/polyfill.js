// Polyfill for Node.js < 20 (toReversed was added in Node 20)
if (!Array.prototype.toReversed) {
  Array.prototype.toReversed = function() {
    return [...this].reverse();
  };
}
