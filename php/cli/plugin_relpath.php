<?php
// plugin_relpath.php — print the tree-relative install path for a plugin
// component (frankenstyle "type_name"), resolved via Moodle's own
// core_component so the public/ split and every plugin type are handled
// correctly. Read-only; used when adding a plugin from a git repo.
//
//   php mdl-demo/cli/plugin_relpath.php --component=mod_foo

define('CLI_SCRIPT', true);

require(__DIR__ . '/../../config.php');
require_once($CFG->libdir . '/clilib.php');

list($options, $unrecognised) = cli_get_params(['component' => ''], []);
if ($unrecognised) {
    cli_error('unrecognised options: ' . implode(', ', $unrecognised));
}

$component = trim($options['component']);
if ($component === '') {
    cli_error('missing --component');
}

list($type, $name) = core_component::normalize_component($component);
if ($type === 'core' || $name === null || $name === '') {
    cli_error('not a plugin component: ' . $component);
}

// Use the plugin TYPE's base directory + the plugin name — get_plugin_directory
// only knows already-installed plugins, but this plugin is not installed yet.
$types = core_component::get_plugin_types();  // [type => absolute base dir]
if (!isset($types[$type])) {
    cli_error('unknown plugin type: ' . $type);
}

// Tree-relative path from the code root; the base dir is absolute.
$dir = $types[$type] . '/' . $name;
echo ltrim(substr($dir, strlen($CFG->dirroot)), '/');
