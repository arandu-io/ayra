//go:build android

package shell

import "github.com/arandu-io/ayra/internal/jni"

// animatorScale is the system setting the animation scale of the whole device is
// kept under: Settings.Global.ANIMATOR_DURATION_SCALE. Developer options set it,
// and so does the accessibility option "Remove animations", which turns it to 0.
const animatorScale = "animator_duration_scale"

// systemReduceMotion reads the animation scale and answers true when it is zero.
//
// The setting is a float kept as text, and Settings.Global.getString is what
// answers it without a Java half of this package: getInt would take a "0.0" for
// something it cannot read and answer its default. Nothing set is the default
// scale, 1, and so false. A virtual machine that is not there yet, or a call
// that fails, answers false as well: a person who asked for less movement and is
// not heard for a second is better off than one whose application is stopped.
func systemReduceMotion() bool {
	reduced := false
	_ = jni.Do(func(env jni.Env) error {
		contextClass, err := env.Class("android.content.Context")
		if err != nil {
			return err
		}
		global, err := env.Class("android.provider.Settings$Global")
		if err != nil {
			return err
		}

		resolver, err := env.CallObject(jni.Ref(jni.Context()), contextClass,
			"getContentResolver", "()Landroid/content/ContentResolver;")
		if err != nil || resolver == 0 {
			return err
		}
		defer env.DeleteLocal(resolver)

		name := env.String(animatorScale)
		defer env.DeleteLocal(name)
		value, err := env.CallStaticObject(global, "getString",
			"(Landroid/content/ContentResolver;Ljava/lang/String;)Ljava/lang/String;",
			jni.Obj(resolver), jni.Obj(name))
		if err != nil {
			return err
		}
		defer env.DeleteLocal(value)

		reduced = animationsOff(env.GoString(value))
		return nil
	})
	return reduced
}
