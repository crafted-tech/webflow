// i18n.js - Frontend translation system for webflow
// Translations are injected by the backend from assets/translations.json

(function() {
    'use strict';

    const TRANSLATION_PREFIX = '\x01';
    const ARG_SEPARATOR = '\x02';

    // libraryTranslations is injected by backend before this script loads
    // appTranslations is also injected by backend (may be empty object)

    // Merged translations (library + app)
    const translations = {};

    // Current language (persisted to localStorage)
    let currentLang = 'en';
    try {
        currentLang = localStorage.getItem('webflow_lang') || 'en';
    } catch (e) {
        // localStorage may not be available in some contexts (WebViews, private browsing)
    }

    // Initialize translations by merging library and app translations
    function initTranslations() {
        // Start with library translations (injected by backend)
        if (typeof libraryTranslations !== 'undefined' && libraryTranslations) {
            for (const lang in libraryTranslations) {
                translations[lang] = { ...libraryTranslations[lang] };
            }
        }
        // Merge app translations (if provided by backend)
        if (typeof appTranslations !== 'undefined' && appTranslations) {
            for (const lang in appTranslations) {
                if (translations[lang]) {
                    translations[lang] = { ...translations[lang], ...appTranslations[lang] };
                } else {
                    translations[lang] = { ...appTranslations[lang] };
                }
            }
        }
    }

    // Translate a single text string
    function translate(text) {
        if (!text || typeof text !== 'string') {
            return text;
        }

        if (!text.startsWith(TRANSLATION_PREFIX)) {
            return text; // Literal text - return as-is
        }

        // Strip prefix and parse key + optional args
        const content = text.slice(1); // Remove \x01
        let key, args = [];

        const sepIndex = content.indexOf(ARG_SEPARATOR);
        if (sepIndex >= 0) {
            key = content.slice(0, sepIndex);
            try {
                args = JSON.parse(content.slice(sepIndex + 1));
                // Recursively translate any args that are themselves translation keys
                args = args.map(function(arg) {
                    if (typeof arg === 'string' && arg.startsWith(TRANSLATION_PREFIX)) {
                        return translate(arg);
                    }
                    return arg;
                });
            } catch (e) {
                // Ignore parse errors
            }
        } else {
            key = content;
        }

        // Get template string with fallback chain: current lang -> English -> raw key
        let template = translations[currentLang]?.[key]
                    || translations['en']?.[key]
                    || key;

        // Substitute placeholders: {0}, {1}, etc.
        return template.replace(/\{(\d+)\}/g, function(match, index) {
            const idx = parseInt(index, 10);
            return (args[idx] !== undefined) ? args[idx] : match;
        });
    }

    // Parse a translation string into key and args
    function parseTranslationString(text) {
        if (!text || !text.startsWith(TRANSLATION_PREFIX)) {
            return null;
        }
        const content = text.slice(1);
        const sepIndex = content.indexOf(ARG_SEPARATOR);
        if (sepIndex >= 0) {
            return {
                key: content.slice(0, sepIndex),
                args: content.slice(sepIndex + 1)
            };
        }
        return { key: content, args: null };
    }

    // Translate using parsed key/args object
    function translateParsed(parsed) {
        if (!parsed) return '';

        let args = [];
        if (parsed.args) {
            try {
                args = JSON.parse(parsed.args);
                // Recursively translate any args that are themselves translation keys
                args = args.map(function(arg) {
                    if (typeof arg === 'string' && arg.startsWith(TRANSLATION_PREFIX)) {
                        return translate(arg);
                    }
                    return arg;
                });
            } catch (e) { /* ignore */ }
        }

        let template = translations[currentLang]?.[parsed.key]
                    || translations['en']?.[parsed.key]
                    || parsed.key;

        return template.replace(/\{(\d+)\}/g, function(match, index) {
            const idx = parseInt(index, 10);
            return (args[idx] !== undefined) ? args[idx] : match;
        });
    }

    // Translate all text nodes and attributes in the page
    function translatePage() {
        // Walk DOM and translate text nodes containing prefix
        const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
        while (walker.nextNode()) {
            const node = walker.currentNode;
            if (node.textContent && node.textContent.includes(TRANSLATION_PREFIX)) {
                // Parse and store as JSON for reliable round-trip through HTML attributes
                const parsed = parseTranslationString(node.textContent);
                if (parsed) {
                    const parent = node.parentElement;
                    if (parent && !parent.hasAttribute('data-i18n')) {
                        parent.setAttribute('data-i18n', JSON.stringify(parsed));
                    }
                    node.textContent = translateParsed(parsed);
                }
            }
        }

        // Translate placeholder and title attributes
        document.querySelectorAll('[placeholder], [title]').forEach(function(el) {
            if (el.placeholder && el.placeholder.startsWith(TRANSLATION_PREFIX)) {
                const parsed = parseTranslationString(el.placeholder);
                if (parsed) {
                    el.setAttribute('data-i18n-placeholder', JSON.stringify(parsed));
                    el.placeholder = translateParsed(parsed);
                }
            }
            if (el.title && el.title.startsWith(TRANSLATION_PREFIX)) {
                const parsed = parseTranslationString(el.title);
                if (parsed) {
                    el.setAttribute('data-i18n-title', JSON.stringify(parsed));
                    el.title = translateParsed(parsed);
                }
            }
        });
    }

    // Re-translate page using stored keys (for language change)
    function retranslatePage() {
        // Re-translate elements with stored keys
        document.querySelectorAll('[data-i18n]').forEach(function(el) {
            try {
                const parsed = JSON.parse(el.getAttribute('data-i18n'));
                el.textContent = translateParsed(parsed);
            } catch (e) { /* ignore malformed */ }
        });

        // Re-translate attributes
        document.querySelectorAll('[data-i18n-placeholder]').forEach(function(el) {
            try {
                const parsed = JSON.parse(el.getAttribute('data-i18n-placeholder'));
                el.placeholder = translateParsed(parsed);
            } catch (e) { /* ignore */ }
        });
        document.querySelectorAll('[data-i18n-title]').forEach(function(el) {
            try {
                const parsed = JSON.parse(el.getAttribute('data-i18n-title'));
                el.title = translateParsed(parsed);
            } catch (e) { /* ignore */ }
        });
    }

    // Set current language
    function setLanguage(lang) {
        if (translations[lang]) {
            currentLang = lang;
            try {
                localStorage.setItem('webflow_lang', lang);
            } catch (e) {
                // localStorage may not be available in some contexts
            }
        }
    }

    // Get current language
    function getLanguage() {
        return currentLang;
    }

    // Get language name for display
    function getLanguageName(lang) {
        return translations[lang]?.['_name'] || lang;
    }

    // Get available languages
    function getAvailableLanguages() {
        return Object.keys(translations).map(function(lang) {
            return {
                code: lang,
                name: translations[lang]?.['_name'] || lang
            };
        });
    }

    // Expose functions globally
    window.i18n = {
        init: initTranslations,
        translate: translate,
        translatePage: translatePage,
        retranslatePage: retranslatePage,
        setLanguage: setLanguage,
        getLanguage: getLanguage,
        getLanguageName: getLanguageName,
        getAvailableLanguages: getAvailableLanguages,
        TRANSLATION_PREFIX: TRANSLATION_PREFIX,
        ARG_SEPARATOR: ARG_SEPARATOR
    };

    // Note: initTranslations() is called from runtime.js after libraryTranslations and appTranslations are defined
})();
