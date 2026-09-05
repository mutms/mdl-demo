<?php
// export_plugins.php — list the site's additional (non-core) plugins as JSON,
// for the console's Plugins page. Read-only.
//
//   php mdl-demo/cli/export_plugins.php

define('CLI_SCRIPT', true);

require(__DIR__ . '/../../config.php');
require_once($CFG->libdir . '/clilib.php');

list($options, $unrecognised) = cli_get_params([], []);
if ($unrecognised) {
    cli_error('unrecognised options: ' . implode(', ', $unrecognised));
}

$pluginman = core_plugin_manager::instance();
$out = [];
foreach ($pluginman->get_plugins() as $type => $plugins) {
    foreach ($plugins as $name => $plugin) {
        if ($plugin->is_standard()) {
            continue;
        }
        $out[] = [
            'component'   => $plugin->component,
            'type'        => $type,
            'name'        => $name,
            'displayname' => $plugin->displayname,
            // rootdir is an absolute path; the tree-relative form is what
            // mudev and git speak.
            'relpath'     => ltrim(substr($plugin->rootdir, strlen($CFG->dirroot)), '/'),
            'versiondisk' => $plugin->versiondisk,
            'versiondb'   => $plugin->versiondb,
            'release'     => $plugin->release,
            'status'      => $plugin->get_status(),
        ];
    }
}

// Pure JSON on stdout so the console can parse it directly.
echo json_encode($out);
