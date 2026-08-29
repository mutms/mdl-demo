<?php
// installlang.php — install a Moodle language pack and make it the site
// default, run by the console at install time. Core ships no CLI for this
// (tool_langimport is web-only), hence this wrapper around its controller,
// which picks the right pack for the site's version and resets caches.
//
//   php mdl-demo/cli/installlang.php --lang=cs

define('CLI_SCRIPT', true);

require(file_exists(__DIR__ . '/../../config.php')
    ? __DIR__ . '/../../config.php' : __DIR__ . '/../../../config.php');
require_once($CFG->libdir . '/clilib.php');

list($options, $unrecognised) = cli_get_params(['lang' => ''], []);
if ($unrecognised) {
    cli_error('unrecognised options: ' . implode(', ', $unrecognised));
}
$lang = $options['lang'];
if (!preg_match('/^[a-z]{2,3}(_[a-z0-9_]+)?$/', $lang)) {
    cli_error('--lang must be a language pack code such as cs or de');
}

core_php_time_limit::raise();
$controller = new \tool_langimport\controller();
$count = $controller->install_languagepacks($lang);
if ($count === 0) {
    cli_error("language pack '$lang' was not installed: "
        . implode('; ', array_map('strip_tags', $controller->errors ?: ['unknown error'])));
}
set_config('lang', $lang);
get_string_manager()->reset_caches();
cli_writeln("installed language pack '$lang' and made it the site default");
