//declscope:namespace analyzer

package goroutinectx

// AnalyzerWithFlags builds an analyzer with flags of its own, as Analyzer
// is built, so that a test can set them without changing Analyzer.
var AnalyzerWithFlags = analyzerWithFlags
