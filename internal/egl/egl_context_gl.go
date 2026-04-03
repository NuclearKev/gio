// SPDX-License-Identifier: Unlicense OR MIT

//go:build netbsd

package egl

import (
	"errors"
	"fmt"
	"strings"
)

const (
	_EGL_OPENGL_BIT                = 0x8
	_EGL_OPENGL_API                = 0x30A2
	_EGL_CONTEXT_MAJOR_VERSION_KHR = 0x3098
	_EGL_CONTEXT_MINOR_VERSION_KHR = 0x30FB
)

func createContext(disp _EGLDisplay) (*eglContext, error) {
	major, minor, ret := eglInitialize(disp)
	if !ret {
		return nil, fmt.Errorf("eglInitialize failed: 0x%x", eglGetError())
	}

	// Bind to desktop OpenGL API instead of OpenGL ES
	if !eglBindAPI(_EGL_OPENGL_API) {
		return nil, fmt.Errorf("eglBindAPI failed: 0x%x", eglGetError())
	}

	// sRGB framebuffer support on EGL 1.5 or if EGL_KHR_gl_colorspace is supported.
	exts := strings.Split(eglQueryString(disp, _EGL_EXTENSIONS), " ")
	srgb := major > 1 || minor >= 5 || hasExtension(exts, "EGL_KHR_gl_colorspace")
	attribs := []_EGLint{
		_EGL_RENDERABLE_TYPE, _EGL_OPENGL_BIT,
		_EGL_SURFACE_TYPE, _EGL_WINDOW_BIT,
		_EGL_BLUE_SIZE, 8,
		_EGL_GREEN_SIZE, 8,
		_EGL_RED_SIZE, 8,
		_EGL_CONFIG_CAVEAT, _EGL_NONE,
	}
	attribs = append(attribs, _EGL_NONE)
	eglCfg, ret := eglChooseConfig(disp, attribs)
	if !ret {
		return nil, fmt.Errorf("eglChooseConfig failed: 0x%x", eglGetError())
	}
	if eglCfg == nilEGLConfig {
		supportsNoCfg := hasExtension(exts, "EGL_KHR_no_config_context")
		if !supportsNoCfg {
			return nil, errors.New("eglChooseConfig returned no configs")
		}
	}
	var visID _EGLint
	if eglCfg != nilEGLConfig {
		var ok bool
		visID, ok = eglGetConfigAttrib(disp, eglCfg, _EGL_NATIVE_VISUAL_ID)
		if !ok {
			return nil, errors.New("newContext: eglGetConfigAttrib for _EGL_NATIVE_VISUAL_ID failed")
		}
	}

	// Request desktop OpenGL 3.3 core context
	ctxAttribs := []_EGLint{
		_EGL_CONTEXT_MAJOR_VERSION_KHR, 3,
		_EGL_CONTEXT_MINOR_VERSION_KHR, 3,
		_EGL_NONE,
	}
	eglCtx := eglCreateContext(disp, eglCfg, nilEGLContext, ctxAttribs)
	if eglCtx == nilEGLContext {
		// Fall back to OpenGL 3.0
		ctxAttribs = []_EGLint{
			_EGL_CONTEXT_MAJOR_VERSION_KHR, 3,
			_EGL_CONTEXT_MINOR_VERSION_KHR, 0,
			_EGL_NONE,
		}
		eglCtx = eglCreateContext(disp, eglCfg, nilEGLContext, ctxAttribs)
		if eglCtx == nilEGLContext {
			return nil, fmt.Errorf("eglCreateContext failed: 0x%x", eglGetError())
		}
	}
	return &eglContext{
		config:      _EGLConfig(eglCfg),
		ctx:         _EGLContext(eglCtx),
		visualID:    int(visID),
		srgb:        srgb,
		surfaceless: hasExtension(exts, "EGL_KHR_surfaceless_context"),
	}, nil
}
