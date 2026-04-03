// SPDX-License-Identifier: Unlicense OR MIT

//go:build linux || windows || freebsd || openbsd || netbsd
// +build linux windows freebsd openbsd netbsd

package egl

import (
	"errors"
	"fmt"
	"runtime"

	"gioui.org/gpu"
)

type Context struct {
	disp    _EGLDisplay
	eglCtx  *eglContext
	eglSurf _EGLSurface
}

type eglContext struct {
	config      _EGLConfig
	ctx         _EGLContext
	visualID    int
	srgb        bool
	surfaceless bool
}

var (
	nilEGLDisplay       _EGLDisplay
	nilEGLSurface       _EGLSurface
	nilEGLContext       _EGLContext
	nilEGLConfig        _EGLConfig
	EGL_DEFAULT_DISPLAY NativeDisplayType
)

const (
	_EGL_ALPHA_SIZE             = 0x3021
	_EGL_BLUE_SIZE              = 0x3022
	_EGL_CONFIG_CAVEAT          = 0x3027
	_EGL_CONTEXT_CLIENT_VERSION = 0x3098
	_EGL_DEPTH_SIZE             = 0x3025
	_EGL_GL_COLORSPACE_KHR      = 0x309d
	_EGL_GL_COLORSPACE_SRGB_KHR = 0x3089
	_EGL_GREEN_SIZE             = 0x3023
	_EGL_EXTENSIONS             = 0x3055
	_EGL_NATIVE_VISUAL_ID       = 0x302e
	_EGL_NONE                   = 0x3038
	_EGL_OPENGL_ES2_BIT         = 0x4
	_EGL_RED_SIZE               = 0x3024
	_EGL_RENDERABLE_TYPE        = 0x3040
	_EGL_SURFACE_TYPE           = 0x3033
	_EGL_WINDOW_BIT             = 0x4
)

func (c *Context) Release() {
	c.ReleaseSurface()
	if c.eglCtx != nil {
		eglDestroyContext(c.disp, c.eglCtx.ctx)
		c.eglCtx = nil
	}
	eglTerminate(c.disp)
	c.disp = nilEGLDisplay
}

func (c *Context) Present() error {
	if !eglSwapBuffers(c.disp, c.eglSurf) {
		return fmt.Errorf("eglSwapBuffers failed (%x)", eglGetError())
	}
	return nil
}

func NewContext(disp NativeDisplayType) (*Context, error) {
	if err := loadEGL(); err != nil {
		return nil, err
	}
	eglDisp := eglGetDisplay(disp)
	// eglGetDisplay can return EGL_NO_DISPLAY yet no error
	// (EGL_SUCCESS), in which case a default EGL display might be
	// available.
	if eglDisp == nilEGLDisplay {
		eglDisp = eglGetDisplay(EGL_DEFAULT_DISPLAY)
	}
	if eglDisp == nilEGLDisplay {
		return nil, fmt.Errorf("eglGetDisplay failed: 0x%x", eglGetError())
	}
	eglCtx, err := createContext(eglDisp)
	if err != nil {
		return nil, err
	}
	c := &Context{
		disp:   eglDisp,
		eglCtx: eglCtx,
	}
	return c, nil
}

func (c *Context) RenderTarget() (gpu.RenderTarget, error) {
	return gpu.OpenGLRenderTarget{}, nil
}

func (c *Context) API() gpu.API {
	return gpu.OpenGL{}
}

func (c *Context) ReleaseSurface() {
	if c.eglSurf == nilEGLSurface {
		return
	}
	// Make sure any in-flight GL commands are complete.
	eglWaitClient()
	c.ReleaseCurrent()
	eglDestroySurface(c.disp, c.eglSurf)
	c.eglSurf = nilEGLSurface
}

func (c *Context) VisualID() int {
	return c.eglCtx.visualID
}

func (c *Context) CreateSurface(win NativeWindowType) error {
	eglSurf, err := createSurface(c.disp, c.eglCtx, win)
	c.eglSurf = eglSurf
	return err
}

func (c *Context) ReleaseCurrent() {
	if c.disp != nilEGLDisplay {
		eglMakeCurrent(c.disp, nilEGLSurface, nilEGLSurface, nilEGLContext)
	}
}

func (c *Context) MakeCurrent() error {
	// OpenGL contexts are implicit and thread-local. Lock the OS thread.
	runtime.LockOSThread()

	if c.eglSurf == nilEGLSurface && !c.eglCtx.surfaceless {
		return errors.New("no surface created yet EGL_KHR_surfaceless_context is not supported")
	}
	if !eglMakeCurrent(c.disp, c.eglSurf, c.eglSurf, c.eglCtx.ctx) {
		return fmt.Errorf("eglMakeCurrent error 0x%x", eglGetError())
	}
	return nil
}

func (c *Context) EnableVSync(enable bool) {
	if enable {
		eglSwapInterval(c.disp, 1)
	} else {
		eglSwapInterval(c.disp, 0)
	}
}

func hasExtension(exts []string, ext string) bool {
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func createSurface(disp _EGLDisplay, eglCtx *eglContext, win NativeWindowType) (_EGLSurface, error) {
	var surfAttribs []_EGLint
	if eglCtx.srgb {
		surfAttribs = append(surfAttribs, _EGL_GL_COLORSPACE_KHR, _EGL_GL_COLORSPACE_SRGB_KHR)
	}
	surfAttribs = append(surfAttribs, _EGL_NONE)
	eglSurf := eglCreateWindowSurface(disp, eglCtx.config, win, surfAttribs)
	if eglSurf == nilEGLSurface && eglCtx.srgb {
		// Try again without sRGB.
		eglCtx.srgb = false
		surfAttribs = []_EGLint{_EGL_NONE}
		eglSurf = eglCreateWindowSurface(disp, eglCtx.config, win, surfAttribs)
	}
	if eglSurf == nilEGLSurface {
		return nilEGLSurface, fmt.Errorf("newContext: eglCreateWindowSurface failed 0x%x (sRGB=%v)", eglGetError(), eglCtx.srgb)
	}
	return eglSurf, nil
}
