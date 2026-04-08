// commitlint.config.js
// Conventional Commits with a custom scope rule for the kvach project.
//
// Valid commit examples:
//   feat(agent): add streaming event bus
//   fix(tool/bash): handle timeout cancellation
//   docs: update architecture plan
//   chore(deps): bump @commitlint/cli to v19
//
// Invalid commit examples:
//   feat(stuff): do things          <- "stuff" is not a known module
//   fix(TICKET-123): patch bug      <- ticket IDs not used here
//   wip: half done                  <- "wip" is not a valid type

'use strict';

/** Known top-level modules and sub-paths in the kvach codebase. */
const KNOWN_SCOPES = [
  // Core runtime
  'agent',
  'loop',
  'compaction',
  'prompt',

  // Tool system
  'tool',
  'tool/bash',
  'tool/read',
  'tool/write',
  'tool/edit',
  'tool/glob',
  'tool/grep',
  'tool/ls',
  'tool/task',
  'tool/webfetch',
  'tool/websearch',
  'tool/question',
  'tool/todo',
  'tool/skill',
  'tool/multipatch',

  // Provider / LLM
  'provider',
  'provider/anthropic',
  'provider/openai',
  'provider/google',
  'provider/ollama',

  // Cross-cutting systems
  'permission',
  'memory',
  'session',
  'hook',
  'mcp',
  'multiagent',
  'skill',
  'snapshot',
  'config',
  'bus',
  'git',
  'lsp',

  // Interfaces
  'server',
  'tui',
  'cli',

  // Project-level
  'deps',
  'ci',
  'docs',
  'release',
  'agents',
  'skills',
];

module.exports = {
  extends: ['@commitlint/config-conventional'],

  rules: {
    // Enforce scope is one of the known module paths (or absent).
    // Severity 2 = error, 'always' = rule always applies.
    'kvach-scope': [2, 'always'],

    // Keep subject sentence-case or lower-case; no trailing period.
    'subject-case': [2, 'never', ['sentence-case', 'start-case', 'pascal-case', 'upper-case']],
    'subject-full-stop': [2, 'never', '.'],

    // Body and footer must be separated by a blank line.
    'body-leading-blank': [2, 'always'],
    'footer-leading-blank': [2, 'always'],

    // Cap header at 100 chars (72 is classic; 100 fits long Go package paths).
    'header-max-length': [2, 'always', 100],
  },

  plugins: [
    {
      rules: {
        /**
         * kvach-scope: when a scope is present it must be one of the
         * KNOWN_SCOPES paths above. An absent scope is always valid.
         */
        'kvach-scope': ({ scope }) => {
          // No scope provided — perfectly fine.
          if (!scope) return [true];

          const valid = KNOWN_SCOPES.includes(scope);
          return [
            valid,
            `Scope "${scope}" is not recognised.\n` +
              `Known scopes: ${KNOWN_SCOPES.join(', ')}\n` +
              `Add new module paths to commitlint.config.js when you create them.`,
          ];
        },
      },
    },
  ],
};
