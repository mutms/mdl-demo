<?php
// Query runner behind mdl-sql; started through php/run.php, so
// Moodle is loaded and $DB is the project's database.
//
// Arguments: <format> <limit> <write 0|1> <sql>

[$mdlcliformat, $mdlclilimit, $mdlcliwrite, $mdlclisql] = [$argv[1], (int)$argv[2], $argv[3] === '1', trim($argv[4])];
$mdlclisql = rtrim($mdlclisql, "; \t\n\r");
if ($mdlclisql === '') {
    fwrite(STDERR, "mdl-sql: no SQL given\n");
    exit(1);
}

$mdlclireads = (bool)preg_match('/^\s*(select|with|show|explain|values)\b/i', $mdlclisql);

// EXPLAIN and SHOW must reach the database as written: Moodle would append
// LIMIT (explaining a different query, or a syntax error after SHOW) or wrap
// them in a PostgreSQL cursor, which neither can run in.
$mdlcliverbatim = (bool)preg_match('/^\s*(show|explain)\b/i', $mdlclisql);

// EXPLAIN ANALYZE executes its statement, so a change still needs --write.
if (preg_match('/^\s*explain\s+((?:\([^)]*\)\s*|(?:analy[sz]e|verbose|extended|partitions|format\s*=\s*\w+)\s+)*)(.*)$/is', $mdlclisql, $mdlclimatch)) {
    if (preg_match('/\banaly[sz]e\b/i', $mdlclimatch[1]) && !preg_match('/^(select|with|values|table)\b/i', $mdlclimatch[2])) {
        if (!$mdlcliwrite) {
            fwrite(STDERR, "mdl-sql: EXPLAIN ANALYZE runs this statement and it changes data, add --write to run it\n");
            exit(1);
        }
    }
}

if (!$mdlclireads) {
    if (!$mdlcliwrite) {
        fwrite(STDERR, "mdl-sql: this statement changes data, add --write to run it\n");
        exit(1);
    }
    $DB->execute($mdlclisql);
    echo "OK\n";
    exit(0);
}

// A recordset, not get_records_sql(): the first column need not be unique.
if ($mdlcliverbatim) {
    // No LIMIT and no cursor; the row limit is applied below while reading.
    (function () {
        $this->dboptions['fetchbuffersize'] = 0;
    })->call($DB);
    $mdlclirs = $DB->get_recordset_sql($mdlclisql);
} else {
    $mdlclirs = $DB->get_recordset_sql($mdlclisql, null, 0, $mdlclilimit > 0 ? $mdlclilimit + 1 : 0);
}
$mdlclirows = [];
$mdlclimore = false;
foreach ($mdlclirs as $mdlclirow) {
    if ($mdlclilimit > 0 && count($mdlclirows) >= $mdlclilimit) {
        $mdlclimore = true;
        break;
    }
    $mdlclirows[] = (array)$mdlclirow;
}
$mdlclirs->close();

if ($mdlcliformat === 'json') {
    echo json_encode($mdlclirows, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES), "\n";
} else if (!$mdlclirows) {
    echo "(no rows)\n";
} else {
    $mdlclicell = function ($value) use ($mdlcliformat, $mdlcliverbatim): string {
        if ($value === null) {
            return 'NULL';
        }
        $value = str_replace(["\r", "\n", "\t"], ['\r', '\n', '\t'], (string)$value);
        if ($mdlcliformat === 'table' && !$mdlcliverbatim && mb_strlen($value) > 60) {
            $value = mb_substr($value, 0, 57) . '...';
        }
        return $value;
    };
    $mdlclilines = [array_keys($mdlclirows[0])];
    foreach ($mdlclirows as $mdlclirow) {
        $mdlclilines[] = array_map($mdlclicell, array_values($mdlclirow));
    }
    if ($mdlcliformat === 'tsv') {
        foreach ($mdlclilines as $mdlcliline) {
            echo implode("\t", $mdlcliline), "\n";
        }
    } else {
        $mdlcliwidths = [];
        foreach ($mdlclilines as $mdlcliline) {
            foreach ($mdlcliline as $i => $value) {
                $mdlcliwidths[$i] = max($mdlcliwidths[$i] ?? 0, mb_strlen($value));
            }
        }
        foreach ($mdlclilines as $n => $mdlcliline) {
            $out = [];
            foreach ($mdlcliline as $i => $value) {
                $out[] = $value . str_repeat(' ', $mdlcliwidths[$i] - mb_strlen($value));
            }
            echo rtrim(implode(' | ', $out)), "\n";
            if ($n === 0) {
                echo implode('-+-', array_map(fn($w) => str_repeat('-', $w), $mdlcliwidths)), "\n";
            }
        }
    }
}

if ($mdlclimore) {
    fwrite(STDERR, "(more rows exist; showing the first {$mdlclilimit}, raise --limit or use --limit=0)\n");
}
