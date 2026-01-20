// Flow runtime - handles UI interactions and Go communication

(function() {
    'use strict';

    // Track input modality (keyboard vs mouse) for focus styling
    // Based on focus-visible polyfill approach
    var hadKeyboardEvent = false;

    document.addEventListener('keydown', function(e) {
        if (e.metaKey || e.altKey || e.ctrlKey) return;
        hadKeyboardEvent = true;
        document.body.classList.add('using-keyboard');
    }, true);

    document.addEventListener('mousedown', function() {
        hadKeyboardEvent = false;
        document.body.classList.remove('using-keyboard');
    }, true);

    document.addEventListener('pointerdown', function() {
        hadKeyboardEvent = false;
        document.body.classList.remove('using-keyboard');
    }, true);

    // Send message to Go
    function sendMessage(type, data) {
        const message = JSON.stringify({ type: type, ...data });
        // Try WebView2 native postMessage first, then fallback to bridge
        if (window.chrome && window.chrome.webview && window.chrome.webview.postMessage) {
            window.chrome.webview.postMessage(message);
        } else if (window.external && window.external.invoke) {
            window.external.invoke(message);
        }
    }

    // Handle button clicks
    document.addEventListener('click', function(e) {
        var button = e.target.closest('[data-button]');
        if (button) {
            e.preventDefault();
            var buttonId = button.getAttribute('data-button');

            // Handle special action buttons locally
            if (buttonId === 'review_copy') {
                window.copyReviewContent();
                return;
            }
            if (buttonId === 'review_save') {
                window.saveReviewContent();
                return;
            }

            // Collect form data if present
            var formData = collectFormData();

            sendMessage('button_click', {
                button: buttonId,
                data: formData
            });
        }
    });

    // Handle menu item clicks
    document.addEventListener('click', function(e) {
        const menuItem = e.target.closest('.menu-item');
        if (menuItem) {
            e.preventDefault();
            const index = parseInt(menuItem.getAttribute('data-index'), 10);

            sendMessage('button_click', {
                button: 'menu_item',
                data: {
                    '_selected_index': index
                }
            });
        }
    });

    // Collect form data
    function collectFormData() {
        const data = {};

        // Text inputs, password inputs, textareas
        document.querySelectorAll('input[type="text"], input[type="password"], textarea').forEach(function(input) {
            if (input.id) {
                data[input.id] = input.value;
            }
        });

        // Checkboxes
        document.querySelectorAll('input[type="checkbox"]').forEach(function(input) {
            if (input.id) {
                data[input.id] = input.checked;
            }
        });

        // Selects
        document.querySelectorAll('select').forEach(function(select) {
            if (select.id) {
                data[select.id] = select.value;
            }
        });

        // Selected radio (single choice)
        const selectedRadio = document.querySelector('.choice-list input[type="radio"]:checked');
        if (selectedRadio) {
            data['_selected_choice'] = selectedRadio.value;
            data['_selected_index'] = parseInt(selectedRadio.getAttribute('data-index'), 10);
        }

        // Selected checkboxes (multi-choice)
        const selectedCheckboxes = document.querySelectorAll('.choice-list-multi input[type="checkbox"]:checked');
        if (selectedCheckboxes.length > 0) {
            const indices = [];
            const values = [];
            selectedCheckboxes.forEach(function(cb) {
                indices.push(parseInt(cb.getAttribute('data-index'), 10));
                values.push(cb.value);
            });
            data['_selected_indices'] = indices;
            data['_selected_values'] = values;
        }

        return data;
    }

    // Update progress bar (called from Go)
    window.updateProgress = function(percent, status) {
        const bar = document.querySelector('.progress-bar');
        const statusEl = document.querySelector('.progress-status');

        if (bar) {
            bar.style.width = percent + '%';
        }
        if (statusEl && status) {
            statusEl.textContent = status;
        }
    };

    // Log view functions (called from Go)
    window.logWriteLine = function(text, styleClass) {
        const logContent = document.getElementById('log-content');
        if (!logContent) return;

        const line = document.createElement('div');
        line.className = 'log-line' + (styleClass ? ' ' + styleClass : '');
        line.textContent = text;
        logContent.appendChild(line);

        // Auto-scroll to bottom
        logContent.scrollTop = logContent.scrollHeight;
    };

    window.logClear = function() {
        const logContent = document.getElementById('log-content');
        if (logContent) {
            logContent.innerHTML = '';
        }
    };

    window.logSetStatus = function(status) {
        const statusEl = document.getElementById('log-status');
        if (statusEl) {
            statusEl.textContent = status;
        }
    };

    // File list functions (called from Go)
    var fileListItems = {}; // Track file elements by path

    window.fileListAddFile = function(path, statusClass, iconSvg) {
        const content = document.getElementById('filelist-content');
        if (!content) return;

        const item = document.createElement('div');
        item.className = 'filelist-item';
        item.setAttribute('data-path', path);

        const icon = document.createElement('div');
        icon.className = 'filelist-icon ' + statusClass;
        icon.innerHTML = iconSvg;

        const pathEl = document.createElement('div');
        pathEl.className = 'filelist-path';
        pathEl.textContent = path;

        item.appendChild(icon);
        item.appendChild(pathEl);
        content.appendChild(item);

        fileListItems[path] = item;

        // Auto-scroll to bottom
        content.scrollTop = content.scrollHeight;
    };

    window.fileListUpdateFile = function(path, statusClass, iconSvg) {
        const item = fileListItems[path];
        if (!item) return;

        const icon = item.querySelector('.filelist-icon');
        if (icon) {
            icon.className = 'filelist-icon ' + statusClass;
            icon.innerHTML = iconSvg;
        }
    };

    window.fileListSetCurrent = function(path) {
        // Remove current class from all items
        document.querySelectorAll('.filelist-item.current').forEach(function(item) {
            item.classList.remove('current');
        });
        // Add current class to specified item
        const item = fileListItems[path];
        if (item) {
            item.classList.add('current');
        }
    };

    window.fileListSetProgress = function(text) {
        const progressEl = document.getElementById('filelist-progress');
        if (progressEl) {
            progressEl.textContent = text;
        }
    };

    window.fileListSetStatus = function(status) {
        const statusEl = document.getElementById('filelist-status');
        if (statusEl) {
            statusEl.textContent = status;
        }
    };

    // Review functions - icons for swap animation
    var iconCopy = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>';
    var iconCheck = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>';

    // Copy text to clipboard - use execCommand for reliability in WebView2
    function copyToClipboard(text, onSuccess) {
        var textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '-9999px';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        try {
            document.execCommand('copy');
            // Always call onSuccess - execCommand return value is unreliable
            if (onSuccess) onSuccess();
        } catch (err) {
            console.error('Copy failed:', err);
        }
        document.body.removeChild(textarea);
    }

    function showCopySuccess(btn) {
        if (!btn) return;
        var iconWrap = btn.querySelector('.btn-icon-wrap');
        if (!iconWrap) return;

        // Stage 1: Fade out current icon
        btn.classList.add('fading');

        // Stage 2: After fade out, swap icon and show success state
        setTimeout(function() {
            iconWrap.innerHTML = iconCheck;
            btn.classList.remove('fading');
            btn.classList.add('copied');

            // Stage 3: After delay, fade out and restore original icon
            setTimeout(function() {
                btn.classList.add('fading');

                setTimeout(function() {
                    iconWrap.innerHTML = iconCopy;
                    btn.classList.remove('fading');
                    btn.classList.remove('copied');
                }, 150);
            }, 1500);
        }, 150);
    }

    window.copyReviewContent = function() {
        var content = document.querySelector('.review-content');
        if (!content) return;

        var text = content.textContent;
        var btn = document.querySelector('[data-button="review_copy"]');

        copyToClipboard(text, function() {
            showCopySuccess(btn);
            // Notify Go that copy was clicked
            sendMessage('button_click', {
                button: 'review_copy',
                data: {}
            });
        });
    };

    window.saveReviewContent = function() {
        // Notify Go to handle save (Go will show file dialog)
        sendMessage('button_click', {
            button: 'review_save',
            data: {}
        });
    };

    // Browse for folder (called from Browse button onclick)
    window.browseFolder = function(targetInputId, title) {
        sendMessage('browse_folder', {
            data: {
                target: targetInputId,
                title: title || 'Select Folder'
            }
        });
    };

    // Focus management on page load
    // If page has focusable content, focus first content element
    // If page has no focusable content, focus the primary/default button
    function initFocus() {
        // Look for focusable content elements (not in footer)
        var contentFocusable = document.querySelector(
            '.flow-content input:not([type="hidden"]), ' +
            '.flow-content select, ' +
            '.flow-content textarea, ' +
            '.flow-content button, ' +
            '.flow-content [tabindex]:not([tabindex="-1"])'
        );

        if (contentFocusable) {
            contentFocusable.focus();
        } else {
            // No focusable content, focus primary button or first button in footer
            var primaryBtn = document.querySelector('.flow-footer .btn-primary');
            if (primaryBtn) {
                primaryBtn.focus();
            } else {
                var firstBtn = document.querySelector('.flow-footer .btn');
                if (firstBtn) {
                    firstBtn.focus();
                }
            }
        }
    }

    // Run focus logic immediately if DOM is ready, otherwise wait
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            initFocus();
            // Notify Go that page is ready for focus
            sendMessage('page_ready', {});
        });
    } else {
        // DOM already ready (script is at end of body)
        initFocus();
        // Notify Go that page is ready for focus
        sendMessage('page_ready', {});
    }

    // Get all visible buttons in footer for arrow navigation
    function getFooterButtons() {
        return Array.from(document.querySelectorAll('.flow-footer .btn')).filter(function(btn) {
            return btn.offsetParent !== null; // visible
        });
    }

    // Get all choice items (radio or checkbox inputs) in a choice list
    function getChoiceInputs(choiceList) {
        return Array.from(choiceList.querySelectorAll('input[type="radio"], input[type="checkbox"]'));
    }

    // Keyboard shortcuts
    document.addEventListener('keydown', function(e) {
        // Arrow Up/Down for choice list navigation
        if ((e.key === 'ArrowUp' || e.key === 'ArrowDown') &&
            (e.target.type === 'radio' || e.target.type === 'checkbox')) {
            var choiceList = e.target.closest('.choice-list, .choice-list-multi');
            if (choiceList) {
                var inputs = getChoiceInputs(choiceList);
                var currentIndex = inputs.indexOf(e.target);
                if (currentIndex !== -1) {
                    var nextIndex;
                    if (e.key === 'ArrowUp') {
                        nextIndex = currentIndex > 0 ? currentIndex - 1 : inputs.length - 1;
                    } else {
                        nextIndex = currentIndex < inputs.length - 1 ? currentIndex + 1 : 0;
                    }
                    inputs[nextIndex].focus();
                    // For radio buttons, also select the focused item
                    if (e.target.type === 'radio') {
                        inputs[nextIndex].checked = true;
                    }
                    e.preventDefault();
                }
            }
        }

        // Arrow Left/Right for button navigation when a button is focused
        if ((e.key === 'ArrowLeft' || e.key === 'ArrowRight') &&
            e.target.classList.contains('btn') &&
            e.target.closest('.flow-footer')) {
            var buttons = getFooterButtons();
            var currentIndex = buttons.indexOf(e.target);
            if (currentIndex !== -1) {
                var nextIndex;
                if (e.key === 'ArrowLeft') {
                    nextIndex = currentIndex > 0 ? currentIndex - 1 : buttons.length - 1;
                } else {
                    nextIndex = currentIndex < buttons.length - 1 ? currentIndex + 1 : 0;
                }
                buttons[nextIndex].focus();
                e.preventDefault();
            }
        }

        // Enter key triggers primary button (unless in textarea or on a button)
        if (e.key === 'Enter' && e.target.tagName !== 'TEXTAREA' && e.target.tagName !== 'BUTTON') {
            var primaryBtn = document.querySelector('.btn-primary[data-button]');
            if (primaryBtn) {
                e.preventDefault();
                primaryBtn.click();
            }
        }

        // Escape key triggers cancel/close button
        if (e.key === 'Escape') {
            var cancelBtn = document.querySelector('[data-button="cancel"], [data-button="close"]');
            if (cancelBtn) {
                e.preventDefault();
                cancelBtn.click();
            }
        }

        // Shift+F5 toggles theme (dev shortcut)
        if (e.key === 'F5' && e.shiftKey) {
            e.preventDefault();
            sendMessage('toggle_theme', {});
        }
    });
})();
